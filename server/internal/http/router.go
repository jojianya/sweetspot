package http

import (
	"context"
	"errors"
	"log/slog"
	stdhttp "net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/config"
	"github.com/jojianya/sweetspot247-backend/internal/di"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
	"github.com/jojianya/sweetspot247-backend/internal/modules/auth"
	"github.com/jojianya/sweetspot247-backend/internal/modules/collections"
	"github.com/jojianya/sweetspot247-backend/internal/modules/comments"
	"github.com/jojianya/sweetspot247-backend/internal/modules/favorites"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	"github.com/jojianya/sweetspot247-backend/internal/modules/reactions"
	"github.com/jojianya/sweetspot247-backend/internal/modules/realtime"
	"github.com/jojianya/sweetspot247-backend/internal/modules/reports"
	"github.com/jojianya/sweetspot247-backend/internal/modules/social"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/internal/observability/report"
)

// quarantineStatus is "ok" or "unwritable" as probed once at boot (see
// app.Run): whether hidden pins' files can actually leave /uploads. It rides
// along in /ready without affecting its HTTP status — unlike DB/Redis, a
// broken quarantine must be loud but must not take all traffic down.
func NewRouter(cfg *config.Config, pool *pgxpool.Pool, c *di.Container, lg *slog.Logger, rep *report.Reporter, quarantineStatus string) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	// TrustedProxies comes from TRUSTED_PROXIES and is validated at startup.
	// nginx proxies /api/*, /events and /uploads/* straight to Go and
	// overwrites X-Forwarded-For with the real peer, so ClientIP() sees the
	// real client. Only list proxy addresses/CIDRs you operate.
	// The list is validated in config.Load, so a failure here means a bug in
	// that validation; refuse to boot rather than run with the wrong peers.
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		panic("set trusted proxies: " + err.Error())
	}
	r.Use(middleware.Recover(rep), middleware.RequestLogger(lg, "/health"), middleware.ReportErrors(rep), middleware.CORS(cfg.CORSAllowedOrigins...), middleware.SecurityHeaders())

	r.Static("/uploads", "./uploads")

	// The DI container is shadowed by the gin handler's *gin.Context in the
	// other closures below, so capture the blacklist here by name.
	blacklist := c.Blacklist

	r.GET("/health", func(c *gin.Context) {
		// Liveness: the process is alive. Deliberately dependency-free so a
		// Redis blip cannot mark a healthy process for restart.
		response.JSON(c, stdhttp.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/ready", func(c *gin.Context) {
		// Readiness: can this instance serve users right now? Pings both
		// backing services. Redis is not optional: the auth middleware
		// consults the session blacklist on every protected request, so a
		// Redis outage is an authentication outage. Reporting "ok" while
		// Redis is unreachable would keep routing users to a server that
		// cannot verify sessions. Uptime monitors and load balancers should
		// watch /ready; container restarts key off /health.
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		dbStatus := "connected"
		if err := pool.Ping(ctx); err != nil {
			dbStatus = "unreachable"
		}

		redisStatus := "connected"
		if blacklist == nil {
			redisStatus = "not_configured"
		} else if err := blacklist.Ping(ctx); err != nil {
			redisStatus = "unreachable"
		}

		if dbStatus != "connected" || redisStatus == "unreachable" {
			response.JSON(c, stdhttp.StatusServiceUnavailable, gin.H{
				"status":     "degraded",
				"db":         dbStatus,
				"redis":      redisStatus,
				"quarantine": quarantineStatus,
			})
			return
		}
		response.JSON(c, stdhttp.StatusOK, gin.H{"status": "ok", "db": dbStatus, "redis": redisStatus, "quarantine": quarantineStatus})
	})

	jsonRoutes := r.Group("")
	jsonRoutes.Use(middleware.BodyLimit(1 << 20))

	// POST /errors ingests client-side crash reports (ErrorBoundary + unhandled
	// window errors) through the same reporter as server errors. Public and
	// best-effort: malformed or oversized payloads are dropped quietly (204) so
	// a broken client can never turn reporting itself into a failure.
	// Rate-limited per IP so a bot cannot flood Sentry/logs at line rate;
	// excess hits get 429 while well-behaved and malformed reports still get
	// 204. The client reporter (client/src/lib/monitoring.ts) is
	// fire-and-forget with no retry, so a 429 never loops.
	errorsLimit := middleware.New(30, time.Minute)
	jsonRoutes.POST("/errors", errorsLimit.Middleware(), ClientErrorIngest(rep))

	uploadRoutes := r.Group("")
	uploadRoutes.Use(middleware.BodyLimit(64 << 20))

	authHandler := auth.NewHandler(
		auth.NewService(c.UserService, cfg.JWTSecret),
		c.Blacklist,
		middleware.New(5, time.Minute),
		// Per-account backstop: 20 failures per 15 minutes per identifier
		// across all IPs. High enough that a stranger cannot lock the real
		// owner out, low enough to stop a distributed grind.
		middleware.New(20, 15*time.Minute),
		auth.SameSiteMode(cfg.CookieSameSite),
		cfg.TrustedProxies,
	)
	mailer := selectMailer(cfg)
	auth.RegisterRoutes(jsonRoutes, authHandler, auth.RouteOptions{
		JWTSecret:      cfg.JWTSecret,
		Blacklist:      c.Blacklist,
		CookieSameSite: auth.SameSiteMode(cfg.CookieSameSite),
		Sessions:       c.UserService,
		UserService:    c.UserService,
		ResetStore:     auth.NewResetStore(pool),
		Mailer:         mailer,
		BaseURL:        cfg.PublicBaseURL,
		TrustedProxies: cfg.TrustedProxies,
	})

	userHandler := users.NewHandler(c.UserService, c.Store)
	// Registered on uploadRoutes: PATCH /users/me is multipart (avatar upload).
	users.RegisterRoutes(uploadRoutes, userHandler, users.RouteOptions{JWTSecret: cfg.JWTSecret, Blacklist: c.Blacklist, Sessions: c.UserService})

	// Real-time stream of newly created pins (SSE). Registered before the
	// pin routes so /events never collides with a parameter route.
	realtimeHandler := realtime.NewHandler(c.Events, cfg.MaxSSEConnections)
	jsonRoutes.GET("/events", realtimeHandler.Stream)

	pinHandler := pins.NewHandler(c.PinRepo, c.Store, c.Events, c.UserService)
	pins.RegisterRoutes(uploadRoutes, pinHandler, pins.RouteOptions{
		JWTSecret: cfg.JWTSecret,
		Blacklist: c.Blacklist,
		Sessions:  c.UserService,
		// Only GET /pins/:id answers reacted_by_me. The reaction repository is
		// passed directly: it already exposes ReactedByMe, and handing the pins
		// handler the repository (not the service) keeps the dependency narrow.
		Reactions: c.ReactionRepo,
	})

	reportHandler := reports.NewHandler(reports.NewService(c.ReportRepo), c.Events).
		WithQuarantine(c.Store, c.PinRepo)
	reports.RegisterRoutes(jsonRoutes, reportHandler, reports.RouteOptions{
		JWTSecret:   cfg.JWTSecret,
		Blacklist:   c.Blacklist,
		UserService: c.UserService,
		Sessions:    c.UserService,
	})

	favoriteHandler := favorites.NewHandler(c.FavoriteRepo)
	favorites.RegisterRoutes(jsonRoutes, favoriteHandler, favorites.RouteOptions{
		JWTSecret: cfg.JWTSecret,
		Blacklist: c.Blacklist,
		Sessions:  c.UserService,
	})

	commentHandler := comments.NewHandler(c.CommentRepo, c.UserService)
	comments.RegisterRoutes(jsonRoutes, commentHandler, comments.RouteOptions{JWTSecret: cfg.JWTSecret, Blacklist: c.Blacklist, Sessions: c.UserService})

	reactionHandler := reactions.NewHandler(c.ReactionRepo)
	reactions.RegisterRoutes(jsonRoutes, reactionHandler, reactions.RouteOptions{JWTSecret: cfg.JWTSecret, Blacklist: c.Blacklist, Sessions: c.UserService})

	socialHandler := social.NewHandler(c.SocialRepo)
	social.RegisterRoutes(jsonRoutes, socialHandler, social.RouteOptions{JWTSecret: cfg.JWTSecret, Blacklist: c.Blacklist, Sessions: c.UserService})

	collectionHandler := collections.NewHandler(c.CollectionRepo, c.UserService)
	collections.RegisterRoutes(jsonRoutes, collectionHandler, collections.RouteOptions{JWTSecret: cfg.JWTSecret, Blacklist: c.Blacklist, Sessions: c.UserService})

	return r
}

// selectMailer chooses HTTP email delivery when a webhook is configured and
// the log-only adapter otherwise. Log tokens are enabled outside production
// only, so full reset tokens can never reach production logs through it.
func selectMailer(cfg *config.Config) auth.Mailer {
	if cfg.MailerWebhookURL != "" {
		return auth.WebhookMailer{URL: cfg.MailerWebhookURL, Key: cfg.MailerWebhookKey}
	}
	return auth.LogMailer{LogTokens: cfg.AppEnv != "production"}
}

// Bounds for the POST /errors payload before it reaches the reporter. The
// endpoint is public, so attacker-shaped reports must not produce unbounded
// Sentry events: the message was already capped, and stack, url and extra
// get the same treatment here. The 1 MB body cap and the 30/min limiter are
// the outer bounds; these keep any single report small inside them.
const (
	clientErrorMaxStackRunes      = 8 * 1024
	clientErrorMaxURLRunes        = 2 * 1024
	clientErrorMaxMessageRunes    = 2048
	clientErrorMaxExtraValueRunes = 1024
)

// truncateRunes cuts s to at most max runes, so multi-byte characters are
// never split mid-encoding.
func truncateRunes(s string, max int) string {
	if r := []rune(s); len(r) > max {
		return string(r[:max])
	}
	return s
}

// clientErrorAllowedExtraKeys is the allowlist of `extra` keys that survive
// into the report. The endpoint is public and the payload is attacker-shaped,
// so anything the real client does not send is dropped rather than forwarded
// into logs and Sentry. These four are exactly what client/src/lib/monitoring.ts
// attaches (reportError's context, plus kind for the global handlers).
//
// An allowlist rather than a size cap: capping kept the first N keys, which
// still forwarded N attacker-chosen keys with attacker-chosen names.
var clientErrorAllowedExtraKeys = map[string]bool{
	"kind":   true,
	"method": true,
	"url":    true,
	"status": true,
}

// allowlistExtra keeps only the allowlisted keys, with values truncated and
// scrubbed. An unknown or oversized key is dropped, so a hand-crafted report
// can neither smuggle arbitrary context nor use a key name to smuggle content.
func allowlistExtra(extra map[string]any) map[string]any {
	out := make(map[string]any, len(clientErrorAllowedExtraKeys))
	for k, v := range extra {
		if !clientErrorAllowedExtraKeys[k] {
			continue
		}
		out[k] = capExtraValue(v)
	}
	return out
}

func capExtraValue(v any) any {
	if s, ok := v.(string); ok {
		return scrubPII(truncateRunes(s, clientErrorMaxExtraValueRunes))
	}
	return v
}

// Patterns for the two things a browser error message can carry that must not
// reach a log store: a typed email address, and a credential in a header or a
// query string. Logs are copied widely and read by many people, so anything in
// them is effectively published (practice 9.2).
var (
	clientErrorEmailRe   = regexp.MustCompile(`[\w.+-]+@[\w-]+\.[\w.-]+`)
	clientErrorBearerRe  = regexp.MustCompile(`(?i)\bbearer\s+[\w\-.=+/]+`)
	clientErrorTokenKVRe = regexp.MustCompile(`(?i)\b(token|access_token|api_?key|secret|password|authorization)=([^\s&"']+)`)
)

// scrubPII replaces emails and credentials in s with a marker that keeps the
// shape readable without the value. Applied to every client-supplied string
// before it leaves this handler.
func scrubPII(s string) string {
	s = clientErrorEmailRe.ReplaceAllString(s, "[EMAIL]")
	s = clientErrorBearerRe.ReplaceAllString(s, "Bearer [TOKEN]")
	return clientErrorTokenKVRe.ReplaceAllString(s, `$1=[TOKEN]`)
}

// ClientErrorIngest handles POST /errors: it forwards a client-side error
// report to the reporter under the "client" source so crashes in the browser
// land in the same monitoring pipeline as server errors. It always answers 204
// and never fails the request, even for a malformed body.
func ClientErrorIngest(rep middleware.ErrorReporter) gin.HandlerFunc {
	return func(c *gin.Context) {
		var payload struct {
			Message string         `json:"message"`
			Stack   string         `json:"stack"`
			URL     string         `json:"url"`
			Extra   map[string]any `json:"extra"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.Status(stdhttp.StatusNoContent)
			return
		}
		msg := strings.TrimSpace(payload.Message)
		if msg == "" {
			msg = "client error"
		}
		if r := []rune(msg); len(r) > clientErrorMaxMessageRunes {
			msg = string(r[:clientErrorMaxMessageRunes])
		}
		if rep != nil {
			rep.Report(c.Request.Context(), errors.New(scrubPII(msg)),
				"source", "client",
				"url", scrubPII(truncateRunes(payload.URL, clientErrorMaxURLRunes)),
				"stack", scrubPII(truncateRunes(payload.Stack, clientErrorMaxStackRunes)),
				"extra", allowlistExtra(payload.Extra),
			)
		}
		c.Status(stdhttp.StatusNoContent)
	}
}
