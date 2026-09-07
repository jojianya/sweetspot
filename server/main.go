package main

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/config"
	"github.com/jojianya/sweetspot247-backend/db"
	"github.com/jojianya/sweetspot247-backend/pkg/logger"
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

	lg.Info("server starting", "port", cfg.Port, "log_level", cfg.LogLevel, "log_format", cfg.LogFormat)
	if err := r.Run(":" + cfg.Port); err != nil {
		lg.Error("server error", "error", err.Error())
	}
}