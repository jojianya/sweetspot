package app

import (
	"context"
	stdhttp "net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/config"
	"github.com/jojianya/sweetspot247-backend/internal/di"
	"github.com/jojianya/sweetspot247-backend/internal/http"
	"github.com/jojianya/sweetspot247-backend/internal/modules/reports"
	"github.com/jojianya/sweetspot247-backend/internal/observability/logger"
	"github.com/jojianya/sweetspot247-backend/internal/observability/report"
)

// Server timeouts. Without a read-header deadline a client can hold a
// connection open indefinitely by dribbling headers, so a handful of
// connections pin a goroutine each and exhaust the server (Slowloris).
const (
	// Bounds the header block only, so it does not penalise the 64 MB
	// multipart upload path. The header block is a few hundred bytes, so this
	// is generous even on a high-latency mobile link.
	readHeaderTimeout = 5 * time.Second
	// Bounds how long an idle keep-alive connection is held open waiting for
	// its next request. This does not apply to a response already in flight,
	// so it does not shorten the SSE stream.
	idleTimeout = 60 * time.Second
	// Caps total request header memory. Go defaults to 1 MB.
	maxHeaderBytes = 1 << 20
)

func Run(cfg *config.Config, pool *pgxpool.Pool, rep *report.Reporter) error {
	lg := logger.FromContext(nil)

	container := di.Build(cfg, pool)

	// Prove the quarantine directory can take files before serving traffic.
	// A broken quarantine would silently leave hidden pins public, so any
	// failure is an ERROR (Sentry) plus an unwritable /ready field — but
	// /ready stays 200 and boot continues, because quarantine health must
	// not take all traffic down. The sweep is skipped when unwritable since
	// every move would fail anyway.
	quarantineStatus := "ok"
	if err := container.Store.VerifyQuarantineWritable(); err != nil {
		lg.Error("quarantine dir not writable, hidden pins may stay public",
			"error", err.Error(), "dir", container.Store.QuarantineDir())
		if rep != nil {
			rep.Report(context.Background(), err, "component", "quarantine", "dir", container.Store.QuarantineDir())
		}
		quarantineStatus = "unwritable"
	}

	// Sweep files of already-hidden pins out of /uploads (idempotent,
	// best-effort). This heals pins hidden before quarantine wiring existed
	// and finishes moves that failed at review time. A listing failure or a
	// per-file error is logged, never fatal to boot.
	if quarantineStatus == "ok" {
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			checked, moved, err := reports.SweepHiddenPinFiles(ctx, pool, container.Store, cfg.QuarantineDryRun)
			if err != nil {
				lg.Warn("quarantine sweep failed", "error", err.Error(), "dry_run", cfg.QuarantineDryRun)
				return
			}
			lg.Info("quarantine sweep done", "checked", checked, "moved", moved, "dry_run", cfg.QuarantineDryRun)
		}()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := container.Blacklist.Ping(ctx); err != nil {
		lg.Warn("redis unreachable, logout disable", "addr", cfg.RedisAddr, "error", err.Error())
	} else {
		lg.Info("redis connected", "addr", cfg.RedisAddr)
	}

	router := http.NewRouter(cfg, pool, container, lg, rep, quarantineStatus)

	// WriteTimeout is deliberately unset: the realtime SSE endpoint
	// (internal/modules/realtime/handler.go) holds responses open for the life
	// of the subscription, and any write deadline would sever them.
	srv := &stdhttp.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}

	lg.Info("server starting", "port", cfg.Port, "log_level", cfg.LogLevel, "log_format", cfg.LogFormat)
	if err := serve(srv); err != nil {
		return err
	}

	// Close the shared Redis client after the server has stopped.
	if err := container.Redis.Close(); err != nil {
		lg.Warn("redis close failed", "error", err.Error())
	}
	return nil
}
