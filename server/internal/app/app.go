package app

import (
	"context"
	stdhttp "net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/config"
	"github.com/jojianya/sweetspot247-backend/internal/di"
	"github.com/jojianya/sweetspot247-backend/internal/http"
	"github.com/jojianya/sweetspot247-backend/internal/observability/logger"
	"github.com/jojianya/sweetspot247-backend/internal/observability/report"
)

func Run(cfg *config.Config, pool *pgxpool.Pool, rep *report.Reporter) error {
	lg := logger.FromContext(nil)

	container := di.Build(cfg, pool)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := container.Blacklist.Ping(ctx); err != nil {
		lg.Warn("redis unreachable, logout disable", "addr", cfg.RedisAddr, "error", err.Error())
	} else {
		lg.Info("redis connected", "addr", cfg.RedisAddr)
	}

	router := http.NewRouter(cfg, pool, container, lg, rep)

	srv := &stdhttp.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	lg.Info("server starting", "port", cfg.Port, "log_level", cfg.LogLevel, "log_format", cfg.LogFormat)
	return serve(srv)
}
