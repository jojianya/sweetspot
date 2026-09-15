package main

import (
	"context"
	"time"

	"github.com/jojianya/sweetspot247-backend/config"
	"github.com/jojianya/sweetspot247-backend/internal/http"
	"github.com/jojianya/sweetspot247-backend/internal/observability/logger"
	"github.com/jojianya/sweetspot247-backend/internal/pins"
	"github.com/jojianya/sweetspot247-backend/internal/platform/database"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
	"github.com/jojianya/sweetspot247-backend/internal/reports"
	"github.com/jojianya/sweetspot247-backend/internal/session"
	"github.com/jojianya/sweetspot247-backend/internal/users"
)

func main() {
	cfg := config.Load()

	lg := logger.Init(cfg.LogLevel, cfg.LogFormat)
	logger.SetDefault(lg)

	pool, err := database.Connect(cfg.DSN())
	if err != nil {
		lg.Error("could not connect to database", "error", err.Error())
		panic(err)
	}
	defer pool.Close()

	if err := database.RunMigrations(pool, "internal/platform/database/migrations"); err != nil {
		lg.Error("migration failed", "error", err.Error())
		panic(err)
	}

	userRepo := users.NewRepository(pool)
	pinRepo := pins.NewRepository(pool)
	reportRepo := reports.NewRepository(pool)
	store := storage.NewLocal("./uploads", cfg.StorageBase)

	sessions := session.New(cfg.RedisAddr)
	ctxBG, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := sessions.Ping(ctxBG); err != nil {
		lg.Warn("redis unreachable, logout disable", "addr", cfg.RedisAddr, "error", err.Error())
	} else {
		lg.Info("redis connected", "addr", cfg.RedisAddr)
	}

	r := http.NewRouter(cfg, pool, userRepo, pinRepo, reportRepo, store, sessions, lg)

	lg.Info("server starting", "port", cfg.Port, "log_level", cfg.LogLevel, "log_format", cfg.LogFormat)
	if err := r.Run(":" + cfg.Port); err != nil {
		lg.Error("server error", "error", err.Error())
	}
}