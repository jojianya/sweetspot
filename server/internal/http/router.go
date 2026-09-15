package http

import (
	"context"
	"log/slog"
	stdhttp "net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/config"
	"github.com/jojianya/sweetspot247-backend/internal/di"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
	"github.com/jojianya/sweetspot247-backend/internal/modules/auth"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	"github.com/jojianya/sweetspot247-backend/internal/modules/reports"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
)

func NewRouter(cfg *config.Config, pool *pgxpool.Pool, c *di.Container, lg *slog.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.SetTrustedProxies(nil)
	r.Use(middleware.Recover(), middleware.RequestLogger(lg, "/health"), middleware.CORS(cfg.CORSAllowedOrigins...), middleware.SecurityHeaders())

	r.Static("/uploads", "./uploads")

	r.GET("/health", func(c *gin.Context) {
		dbStatus := "connected"
		if err := pool.Ping(context.Background()); err != nil {
			dbStatus = "unreachable"
			response.JSON(c, stdhttp.StatusServiceUnavailable, gin.H{"status": "ok", "db": dbStatus})
			return
		}
		response.JSON(c, stdhttp.StatusOK, gin.H{"status": "ok", "db": dbStatus})
	})

	jsonRoutes := r.Group("")
	jsonRoutes.Use(middleware.BodyLimit(1 << 20))

	uploadRoutes := r.Group("")
	uploadRoutes.Use(middleware.BodyLimit(64 << 20))

	authHandler := auth.NewHandler(
		auth.NewService(c.UserService, cfg.JWTSecret),
		c.Blacklist,
		middleware.New(5, time.Minute),
	)
	auth.RegisterRoutes(jsonRoutes, authHandler, auth.RouteOptions{JWTSecret: cfg.JWTSecret, Blacklist: c.Blacklist})

	userHandler := users.NewHandler(c.UserService)
	users.RegisterRoutes(jsonRoutes, userHandler, users.RouteOptions{JWTSecret: cfg.JWTSecret, Blacklist: c.Blacklist})

	pinHandler := pins.NewHandler(pins.NewService(c.PinRepo), c.Store)
	pins.RegisterRoutes(uploadRoutes, pinHandler, pins.RouteOptions{JWTSecret: cfg.JWTSecret, Blacklist: c.Blacklist})

	reportHandler := reports.NewHandler(reports.NewService(c.ReportRepo))
	reports.RegisterRoutes(jsonRoutes, reportHandler, reports.RouteOptions{
		JWTSecret:   cfg.JWTSecret,
		Blacklist:   c.Blacklist,
		UserService: c.UserService,
	})

	return r
}
