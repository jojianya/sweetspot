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
	r.POST("/auth/register", registerLimit.Middleware(), auth.NewRegisterHandler(c.UserRepo, cfg.JWTSecret).Handle)

	loginHandler := auth.NewLoginHandler(c.UserRepo, cfg.JWTSecret, middleware.New(5, time.Minute))
	loginIPLimit := middleware.New(20, time.Minute)
	r.POST("/auth/login", loginIPLimit.Middleware(), loginHandler.Handle)

	r.POST("/auth/logout", middleware.AuthRequired(cfg.JWTSecret, c.Blacklist), auth.Logout(c.Blacklist))
	r.GET("/me", middleware.AuthRequired(cfg.JWTSecret, c.Blacklist), auth.Me(c.UserRepo))

	r.GET("/users/:id", validid.Middleware(), users.GetUser(c.UserRepo))
	r.PATCH("/users/:id/role", middleware.AuthRequired(cfg.JWTSecret, c.Blacklist), middleware.RequireOwner(c.UserRepo), validid.Middleware(), users.UpdateRole(c.UserRepo, middleware.GetUserID))

	r.GET("/categories", pins.ListCategories(c.PinRepo))
	r.GET("/pins", pins.GetPins(c.PinRepo))
	r.GET("/pins/:id", validid.Middleware(), middleware.OptionalAuth(cfg.JWTSecret, c.Blacklist), pins.GetPin(c.PinRepo, c.UserRepo))

	r.POST("/pins/:id/report", middleware.AuthRequired(cfg.JWTSecret, c.Blacklist), validid.Middleware(), reports.CreateReport(c.ReportRepo))
	r.GET("/reports", middleware.AuthRequired(cfg.JWTSecret, c.Blacklist), middleware.RequireAdmin(c.UserRepo), reports.ListReports(c.ReportRepo))
	r.PATCH("/reports/:id", middleware.AuthRequired(cfg.JWTSecret, c.Blacklist), middleware.RequireAdmin(c.UserRepo), validid.Middleware(), reports.ReviewReport(c.ReportRepo))

	pinCreateLimit := middleware.New(10, time.Minute)
	r.POST("/pins", pinCreateLimit.Middleware(), middleware.AuthRequired(cfg.JWTSecret, c.Blacklist), pins.CreatePin(c.PinRepo, c.Store))

	return r
}
