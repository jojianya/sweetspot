package report

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
)

// TestReportWithoutDSN stays log-only: with no Sentry DSN configured the
// reporter must not panic, must not try to reach Sentry, and Close must be a
// no-op. This is the default production shape until SENTRY_DSN is set.
func TestReportWithoutDSNLogsOnly(t *testing.T) {
	lg := slog.New(slog.NewTextHandler(io.Discard, nil))
	rep := New(lg, "", "development")

	rep.Report(context.Background(), errors.New("boom"), "path", "/x", "status", 500)
	rep.Close()
}
