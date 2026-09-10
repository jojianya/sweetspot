package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/config"
	"github.com/jojianya/sweetspot247-backend/db"
	"github.com/jojianya/sweetspot247-backend/internal/auth"
	"github.com/jojianya/sweetspot247-backend/internal/users"
	"github.com/jojianya/sweetspot247-backend/pkg/logger"
	"github.com/jojianya/sweetspot247-backend/pkg/ratelimit"
)

func main() {
	cfg := config.Load()

	lg := logger.Init(cfg.LogLevel, cfg.LogFormat)
	logger.SetDefault(lg)

	pool, err := db.Connect(cfg.DSN())
	if err != nil {
		lg.Error("could not connect to database", "error", err.Error())
		panic(err)
	}
	defer pool.Close()

	if err := db.RunMigrations(pool, "migrations"); err != nil {
		lg.Error("migration failed", "error", err.Error())
		panic(err)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()	
	r.Use(gin.Recovery(), logger.RequestLogger(lg, "/health"))

	r.GET("/health", func(c *gin.Context) {
		dbStatus := "connected"
		if err := pool.Ping(context.Background()); err != nil {
			dbStatus = "unreachable"
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "ok", "db": dbStatus})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "db": dbStatus})
	})

	userRepo := users.NewRepository(pool)

	authHandler := auth.NewRegisterHandler(userRepo, cfg.JWTSecret)
	registerLimit := ratelimit.New(5, time.Minute)
	r.POST("/auth/register", registerLimit.Middleware(), authHandler.Handle)

	loginHandler := auth.NewLoginHandler(
		userRepo,
		cfg.JWTSecret,
		ratelimit.New(5, time.Minute),
	)
	loginIPLimit := ratelimit.New(20, time.Minute)
	r.POST("/auth/login", loginIPLimit.Middleware(), loginHandler.Handle)

	r.GET("/me", auth.AuthRequired(cfg.JWTSecret), auth.Me())

	r.GET("/users/:id", users.GetUser(userRepo))

	r.PATCH("/users/:id/role", auth.AuthRequired(cfg.JWTSecret), auth.RequireOwner(userRepo), users.UpdateRole(userRepo))

	lg.Info("server starting", "port", cfg.Port, "log_level", cfg.LogLevel, "log_format", cfg.LogFormat)
	if err := r.Run(":" + cfg.Port); err != nil {
		lg.Error("server error", "error", err.Error())
	}
}