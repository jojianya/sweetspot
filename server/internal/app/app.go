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

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := container.Blacklist.Ping(ctx); err != nil {
		lg.Warn("redis unreachable, logout disable", "addr", cfg.RedisAddr, "error", err.Error())
	} else {
		lg.Info("redis connected", "addr", cfg.RedisAddr)
	}

	router := http.NewRouter(cfg, pool, container, lg, rep)

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
	return serve(srv)
}
