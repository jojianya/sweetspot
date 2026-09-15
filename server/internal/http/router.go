package http

import (
	"context"
	"log/slog"
	stdhttp "net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/config"
	"github.com/jojianya/sweetspot247-backend/internal/auth"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
	"github.com/jojianya/sweetspot247-backend/internal/pins"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
	"github.com/jojianya/sweetspot247-backend/internal/reports"
	"github.com/jojianya/sweetspot247-backend/internal/session"
	"github.com/jojianya/sweetspot247-backend/internal/users"
	"github.com/jojianya/sweetspot247-backend/pkg/validid"
)

func NewRouter(
	cfg *config.Config,
	pool *pgxpool.Pool,
	userRepo *users.Repository,
	pinRepo *pins.Repository,
	reportRepo *reports.Repository,
	store *storage.Local,
	blacklist *session.Blacklist,
	lg *slog.Logger,
) *gin.Engine {
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
	r.POST("/auth/register", registerLimit.Middleware(), auth.NewRegisterHandler(userRepo, cfg.JWTSecret).Handle)

	loginHandler := auth.NewLoginHandler(userRepo, cfg.JWTSecret, middleware.New(5, time.Minute))
	loginIPLimit := middleware.New(20, time.Minute)
	r.POST("/auth/login", loginIPLimit.Middleware(), loginHandler.Handle)

	r.POST("/auth/logout", auth.AuthRequired(cfg.JWTSecret, blacklist), auth.Logout(blacklist))
	r.GET("/me", auth.AuthRequired(cfg.JWTSecret, blacklist), auth.Me(userRepo))

	r.GET("/users/:id", validid.Middleware(), users.GetUser(userRepo))
	r.PATCH("/users/:id/role", auth.AuthRequired(cfg.JWTSecret, blacklist), auth.RequireOwner(userRepo), validid.Middleware(), users.UpdateRole(userRepo, auth.GetUserID))

	r.GET("/categories", pins.ListCategories(pinRepo))
	r.GET("/pins", pins.GetPins(pinRepo))
	r.GET("/pins/:id", validid.Middleware(), auth.OptionalAuth(cfg.JWTSecret, blacklist), pins.GetPin(pinRepo, userRepo))

	r.POST("/pins/:id/report", auth.AuthRequired(cfg.JWTSecret, blacklist), validid.Middleware(), reports.CreateReport(reportRepo))
	r.GET("/reports", auth.AuthRequired(cfg.JWTSecret, blacklist), auth.RequireAdmin(userRepo), reports.ListReports(reportRepo))
	r.PATCH("/reports/:id", auth.AuthRequired(cfg.JWTSecret, blacklist), auth.RequireAdmin(userRepo), validid.Middleware(), reports.ReviewReport(reportRepo))

	pinCreateLimit := middleware.New(10, time.Minute)
	r.POST("/pins", pinCreateLimit.Middleware(), auth.AuthRequired(cfg.JWTSecret, blacklist), pins.CreatePin(pinRepo, store))

	return r
}