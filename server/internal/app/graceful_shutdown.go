package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jojianya/sweetspot247-backend/internal/modules/realtime"
)

func serve(srv *http.Server) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		// ListenAndServe returns ErrServerClosed once Shutdown has run, which
		// is a normal exit rather than a failure.
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		// Close SSE connections first so they don't block shutdown. Without
		// this, srv.Shutdown waits for the full timeout because SSE streams
		// stay open indefinitely.
		if n := realtime.CloseAllSSE(); n > 0 {
			slog.Info("closed SSE connections", "count", n)
		}

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// A graceful stop is the success path: report nil so the process does
		// not log a server error on every SIGTERM.
		_ = srv.Shutdown(shutdownCtx)
		return nil
	}
}
