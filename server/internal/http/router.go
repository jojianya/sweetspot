package http

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/config"
	"github.com/jojianya/sweetspot247-backend/internal/di"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/observability/report"
)

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

	r.GET("/health", livenessHandler())
	r.GET("/ready", readinessHandler(pool, c.Blacklist, quarantineStatus))

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

	registerModuleRoutes(jsonRoutes, uploadRoutes, cfg, c, pool)

	return r
}
