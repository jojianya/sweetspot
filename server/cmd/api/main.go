package main

import (
	"github.com/jojianya/sweetspot247-backend/internal/app"
	"github.com/jojianya/sweetspot247-backend/internal/config"
	"github.com/jojianya/sweetspot247-backend/internal/observability/logger"
	"github.com/jojianya/sweetspot247-backend/internal/platform/database"
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

	if err := database.RunMigrations(pool, database.MigrationsDir); err != nil {
		lg.Error("migration failed", "error", err.Error())
		panic(err)
	}

	if err := app.Run(cfg, pool); err != nil {
		lg.Error("server error", "error", err.Error())
	}
}
