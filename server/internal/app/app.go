package app

import (
	"context"
	"log/slog"
	stdhttp "net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/config"
	"github.com/jojianya/sweetspot247-backend/internal/di"
	"github.com/jojianya/sweetspot247-backend/internal/http"
	"github.com/jojianya/sweetspot247-backend/internal/modules/auth"
	"github.com/jojianya/sweetspot247-backend/internal/modules/reports"
	"github.com/jojianya/sweetspot247-backend/internal/observability/logger"
	"github.com/jojianya/sweetspot247-backend/internal/observability/report"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
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
			checked, moved, failed, err := reports.SweepHiddenPinFiles(ctx, pool, container.Store, cfg.QuarantineDryRun)
			if err != nil {
				lg.Warn("quarantine sweep failed", "error", err.Error(), "dry_run", cfg.QuarantineDryRun)
				return
			}
			logSweepResult(lg, checked, moved, failed, cfg.QuarantineDryRun)
		}()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := container.Blacklist.Ping(ctx); err != nil {
		lg.Warn("redis unreachable, logout disable", "addr", cfg.RedisAddr, "error", err.Error())
	} else {
		lg.Info("redis connected", "addr", cfg.RedisAddr)
	}

	// Periodic resweep heals quarantine moves the immediate path missed
	// without waiting for a restart. One goroutine, stopped when Run
	// returns; passes run sequentially so a slow pass delays the next one
	// instead of overlapping it.
	sweepCtx, stopSweeps := context.WithCancel(context.Background())
	defer stopSweeps()
	if cfg.QuarantineSweepInterval > 0 {
		go sweepLoop(sweepCtx, cfg.QuarantineSweepInterval, func(runCtx context.Context) {
			sweepOnce(runCtx, lg, pool, container.Store, cfg.QuarantineDryRun)
		})
	}

	// Dead reset rows are inert, so their janitor runs on its own (usually
	// daily) cadence beside the quarantine resweep. Same loop shape, same
	// shutdown and no-overlap guarantees.
	resetStore := auth.NewResetStore(pool)
	if cfg.ResetCleanupInterval > 0 {
		go sweepLoop(sweepCtx, cfg.ResetCleanupInterval, func(runCtx context.Context) {
			deleted, err := resetStore.CleanupPasswordResets(runCtx)
			if err != nil {
				lg.Warn("password reset cleanup failed", "error", err.Error())
				return
			}
			if deleted > 0 {
				lg.Info("password reset cleanup done", "deleted", deleted)
			} else {
				lg.Debug("password reset cleanup done", "deleted", 0)
			}
		})
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

// logSweepResult writes one summary line per sweep pass: Warn when anything
// failed, Info when files moved, Debug when the pass was a quiet no-op.
func logSweepResult(lg *slog.Logger, checked, moved, failed int, dryRun bool) {
	attrs := []any{"checked", checked, "moved", moved, "failed", failed, "dry_run", dryRun}
	switch {
	case failed > 0:
		lg.Warn("quarantine sweep done with failures", attrs...)
	case moved > 0:
		lg.Info("quarantine sweep done", attrs...)
	default:
		lg.Debug("quarantine sweep done", attrs...)
	}
}

// sweepLoop runs fn on every tick until ctx is cancelled. Passes never
// overlap: a slow pass delays the next one instead of running beside it.
func sweepLoop(ctx context.Context, interval time.Duration, fn func(context.Context)) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			fn(runCtx)
			cancel()
		}
	}
}

// sweepOnce is one periodic pass: re-probe writability (a fixed permissions
// problem recovers without a restart), skip loudly while unwritable, and
// otherwise sweep with a per-run summary. Shutdown cancels runCtx, which the
// sweep honors, so stopping never waits out the timeout.
func sweepOnce(runCtx context.Context, lg *slog.Logger, pool *pgxpool.Pool, store *storage.Local, dryRun bool) {
	if err := store.VerifyQuarantineWritable(); err != nil {
		lg.Warn("quarantine sweep skipped, dir not writable", "error", err.Error(), "dir", store.QuarantineDir())
		return
	}
	checked, moved, failed, err := reports.SweepHiddenPinFiles(runCtx, pool, store, dryRun)
	if err != nil {
		lg.Warn("quarantine sweep failed", "error", err.Error(), "dry_run", dryRun)
		return
	}
	logSweepResult(lg, checked, moved, failed, dryRun)
}
