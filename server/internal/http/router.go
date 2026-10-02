package http

import (
	"context"
	"errors"
	"log/slog"
	stdhttp "net/http"
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
	"github.com/jojianya/sweetspot247-backend/internal/modules/realtime"
	"github.com/jojianya/sweetspot247-backend/internal/modules/reports"
	"github.com/jojianya/sweetspot247-backend/internal/modules/social"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/internal/observability/report"
)

func NewRouter(cfg *config.Config, pool *pgxpool.Pool, c *di.Container, lg *slog.Logger, rep *report.Reporter) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	// TrustedProxies comes from TRUSTED_PROXIES and is validated at startup.
	// Empty (default) is today's behavior: ClientIP returns the direct peer
	// and X-Forwarded-For is ignored, which is spoof-safe but means per-IP
	// rate limits are shared per proxy behind the Next rewrite (which does not
	// forward X-Forwarded-For). Only list proxy addresses/CIDRs you operate.
	_ = r.SetTrustedProxies(cfg.TrustedProxies)
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
				"status": "degraded",
				"db":     dbStatus,
				"redis":  redisStatus,
			})
			return
		}
		response.JSON(c, stdhttp.StatusOK, gin.H{"status": "ok", "db": dbStatus, "redis": redisStatus})
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
	pins.RegisterRoutes(uploadRoutes, pinHandler, pins.RouteOptions{JWTSecret: cfg.JWTSecret, Blacklist: c.Blacklist, Sessions: c.UserService})

	reportHandler := reports.NewHandler(reports.NewService(c.ReportRepo))
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
		if r := []rune(msg); len(r) > 2048 {
			msg = string(r[:2048])
		}
		if rep != nil {
			rep.Report(c.Request.Context(), errors.New(msg),
				"source", "client",
				"url", payload.URL,
				"stack", payload.Stack,
				"extra", payload.Extra,
			)
		}
		c.Status(stdhttp.StatusNoContent)
	}
}
