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
	r.SetTrustedProxies(nil)
	r.Use(middleware.Recover(rep), middleware.RequestLogger(lg, "/health"), middleware.ReportErrors(rep), middleware.CORS(cfg.CORSAllowedOrigins...), middleware.SecurityHeaders())

	r.Static("/uploads", "./uploads")

	// The DI container is shadowed by the gin handler's *gin.Context in the
	// other closures below, so capture the blacklist here by name.
	blacklist := c.Blacklist

	r.GET("/health", func(c *gin.Context) {
		// Ping both backing services. Redis is not optional: the auth
		// middleware consults the session blacklist on every protected
		// request, so a Redis outage is an authentication outage. Reporting
		// "ok" while Redis is unreachable would let the container healthcheck
		// and any load balancer keep routing to a server that cannot verify
		// sessions.
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
	jsonRoutes.POST("/errors", ClientErrorIngest(rep))

	uploadRoutes := r.Group("")
	uploadRoutes.Use(middleware.BodyLimit(64 << 20))

	authHandler := auth.NewHandler(
		auth.NewService(c.UserService, cfg.JWTSecret),
		c.Blacklist,
		middleware.New(5, time.Minute),
	)
	auth.RegisterRoutes(jsonRoutes, authHandler, auth.RouteOptions{JWTSecret: cfg.JWTSecret, Blacklist: c.Blacklist})

	userHandler := users.NewHandler(c.UserService, c.Store)
	// Registered on uploadRoutes: PATCH /users/me is multipart (avatar upload).
	users.RegisterRoutes(uploadRoutes, userHandler, users.RouteOptions{JWTSecret: cfg.JWTSecret, Blacklist: c.Blacklist})

	// Real-time stream of newly created pins (SSE). Registered before the
	// pin routes so /events never collides with a parameter route.
	realtimeHandler := realtime.NewHandler(c.Events, cfg.MaxSSEConnections)
	jsonRoutes.GET("/events", realtimeHandler.Stream)

	pinHandler := pins.NewHandler(c.PinRepo, c.Store, c.Events, c.UserService)
	pins.RegisterRoutes(uploadRoutes, pinHandler, pins.RouteOptions{JWTSecret: cfg.JWTSecret, Blacklist: c.Blacklist})

	reportHandler := reports.NewHandler(reports.NewService(c.ReportRepo))
	reports.RegisterRoutes(jsonRoutes, reportHandler, reports.RouteOptions{
		JWTSecret:   cfg.JWTSecret,
		Blacklist:   c.Blacklist,
		UserService: c.UserService,
	})

	favoriteHandler := favorites.NewHandler(c.FavoriteRepo)
	favorites.RegisterRoutes(jsonRoutes, favoriteHandler, favorites.RouteOptions{
		JWTSecret: cfg.JWTSecret,
		Blacklist: c.Blacklist,
	})

	commentHandler := comments.NewHandler(c.CommentRepo, c.UserService)
	comments.RegisterRoutes(jsonRoutes, commentHandler, comments.RouteOptions{JWTSecret: cfg.JWTSecret, Blacklist: c.Blacklist})

	socialHandler := social.NewHandler(c.SocialRepo)
	social.RegisterRoutes(jsonRoutes, socialHandler, social.RouteOptions{JWTSecret: cfg.JWTSecret, Blacklist: c.Blacklist})

	collectionHandler := collections.NewHandler(c.CollectionRepo, c.UserService)
	collections.RegisterRoutes(jsonRoutes, collectionHandler, collections.RouteOptions{JWTSecret: cfg.JWTSecret, Blacklist: c.Blacklist})

	return r
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
