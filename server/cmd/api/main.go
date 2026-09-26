package main

import (
	"fmt"
	"os"

	"github.com/jojianya/sweetspot247-backend/internal/app"
	"github.com/jojianya/sweetspot247-backend/internal/config"
	"github.com/jojianya/sweetspot247-backend/internal/observability/logger"
	"github.com/jojianya/sweetspot247-backend/internal/observability/report"
	"github.com/jojianya/sweetspot247-backend/internal/platform/database"
)

// run does the work so that every deferred cleanup executes before the process
// exits. os.Exit in main skips defers, which would leak the database pool and
// the Sentry reporter.
func run() error {
	cfg := config.Load()

	lg := logger.Init(cfg.LogLevel, cfg.LogFormat)
	logger.SetDefault(lg)

	rep := report.New(lg, cfg.SentryDSN, cfg.SentryEnv)
	defer rep.Close()

	pool, err := database.Connect(cfg.DSN())
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	if err := database.RunMigrations(pool, database.MigrationsDir); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	if err := app.Run(cfg, pool, rep); err != nil {
		return fmt.Errorf("serve: %w", err)
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		// The logger is always initialised by the time run can return, so this
		// never falls back to a bare write. Exiting non-zero is what makes
		// Docker restart policies and Kubernetes treat this as a failure
		// instead of a clean stop.
		logger.FromContext(nil).Error("server exited with error", "error", err.Error())
		os.Exit(1)
	}
}
