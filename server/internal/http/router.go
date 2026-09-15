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
	"github.com/jojianya/sweetspot247-backend/pkg/validid"
)

func NewRouter(cfg *config.Config, pool *pgxpool.Pool, c *di.Container, lg *slog.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(middleware.Recover(), middleware.RequestLogger(lg, "/health"), middleware.CORS())

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

	registerLimit := middleware.New(5, time.Minute)
	r.POST("/auth/register", registerLimit.Middleware(), auth.NewRegisterHandler(c.UserService, cfg.JWTSecret).Handle)

	loginHandler := auth.NewLoginHandler(c.UserService, cfg.JWTSecret, middleware.New(5, time.Minute))
	loginIPLimit := middleware.New(20, time.Minute)
	r.POST("/auth/login", loginIPLimit.Middleware(), loginHandler.Handle)

	r.POST("/auth/logout", middleware.AuthRequired(cfg.JWTSecret, c.Blacklist), auth.Logout(c.Blacklist))
	r.GET("/me", middleware.AuthRequired(cfg.JWTSecret, c.Blacklist), auth.Me(c.UserService))

	userHandler := users.NewHandler(c.UserService)
	users.RegisterRoutes(r.Group(""), userHandler, users.RouteOptions{JWTSecret: cfg.JWTSecret, Blacklist: c.Blacklist})

	r.GET("/categories", pins.ListCategories(c.PinRepo))
	r.GET("/pins", pins.GetPins(c.PinRepo))
	r.GET("/pins/:id", validid.Middleware(), middleware.OptionalAuth(cfg.JWTSecret, c.Blacklist), pins.GetPin(c.PinRepo, c.UserService))

	r.POST("/pins/:id/report", middleware.AuthRequired(cfg.JWTSecret, c.Blacklist), validid.Middleware(), reports.CreateReport(c.ReportRepo))
	r.GET("/reports", middleware.AuthRequired(cfg.JWTSecret, c.Blacklist), users.RequireAdmin(c.UserService), reports.ListReports(c.ReportRepo))
	r.PATCH("/reports/:id", middleware.AuthRequired(cfg.JWTSecret, c.Blacklist), users.RequireAdmin(c.UserService), validid.Middleware(), reports.ReviewReport(c.ReportRepo))

	pinCreateLimit := middleware.New(10, time.Minute)
	r.POST("/pins", pinCreateLimit.Middleware(), middleware.AuthRequired(cfg.JWTSecret, c.Blacklist), pins.CreatePin(c.PinRepo, c.Store))

	return r
}
