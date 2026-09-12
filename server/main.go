package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/config"
	"github.com/jojianya/sweetspot247-backend/db"
	"github.com/jojianya/sweetspot247-backend/internal/auth"
	"github.com/jojianya/sweetspot247-backend/internal/pins"
	"github.com/jojianya/sweetspot247-backend/internal/reports"
	"github.com/jojianya/sweetspot247-backend/internal/session"
	"github.com/jojianya/sweetspot247-backend/internal/storage"
	"github.com/jojianya/sweetspot247-backend/internal/users"
	"github.com/jojianya/sweetspot247-backend/pkg/logger"
	"github.com/jojianya/sweetspot247-backend/pkg/ratelimit"
	"github.com/jojianya/sweetspot247-backend/pkg/validid"
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

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:3001", "http://127.0.0.1:3000", "http://127.0.0.1:3001"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.Static("/uploads", "./uploads")

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

	sessions := session.New(cfg.RedisAddr)
	ctxBG, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := sessions.Ping(ctxBG); err != nil {
		lg.Warn("redis unreachable, logout disable", "addr", cfg.RedisAddr, "error", err.Error())
	} else {
		lg.Info("redis connected", "addr", cfg.RedisAddr)
	}

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

	r.POST("/auth/logout", auth.AuthRequired(cfg.JWTSecret, sessions), auth.Logout(sessions))

	r.GET("/me", auth.AuthRequired(cfg.JWTSecret, sessions), auth.Me(userRepo))

	r.GET("/users/:id", validid.Middleware(), users.GetUser(userRepo))

	pinRepo := pins.NewRepository(pool)
	store := storage.NewLocal("./uploads", cfg.StorageBase)
	r.GET("/categories", pins.ListCategories(pinRepo))
	r.GET("/pins", pins.GetPins(pinRepo))
	r.GET("/pins/:id", validid.Middleware(), auth.OptionalAuth(cfg.JWTSecret, sessions), pins.GetPin(pinRepo, userRepo))

	reportRepo := reports.NewRepository(pool)
	r.POST("/pins/:id/report", auth.AuthRequired(cfg.JWTSecret, sessions), validid.Middleware(), reports.CreateReport(reportRepo))
	r.GET("/reports", auth.AuthRequired(cfg.JWTSecret, sessions), auth.RequireAdmin(userRepo), reports.ListReports(reportRepo))
	r.PATCH("/reports/:id", auth.AuthRequired(cfg.JWTSecret, sessions), auth.RequireAdmin(userRepo), validid.Middleware(), reports.ReviewReport(reportRepo))
	pinCreateLimit := ratelimit.New(10, time.Minute)
	r.POST("/pins", pinCreateLimit.Middleware(), auth.AuthRequired(cfg.JWTSecret, sessions), pins.CreatePin(pinRepo, store))

	r.PATCH("/users/:id/role", auth.AuthRequired(cfg.JWTSecret, sessions), auth.RequireOwner(userRepo), validid.Middleware(), users.UpdateRole(userRepo, auth.GetUserID))

	lg.Info("server starting", "port", cfg.Port, "log_level", cfg.LogLevel, "log_format", cfg.LogFormat)
	if err := r.Run(":" + cfg.Port); err != nil {
		lg.Error("server error", "error", err.Error())
	}
}
