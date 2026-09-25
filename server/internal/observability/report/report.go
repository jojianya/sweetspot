// Package report forwards unexpected errors to the structured logger and, when
// a Sentry DSN is configured, to Sentry. It is the downstream hook for the
// existing slog-based logging: middleware and the client error-ingest endpoint
// report through it, so one place owns where errors go.
package report

import (
	"context"
	"log/slog"
	"time"

	"github.com/getsentry/sentry-go"
)

// Reporter is safe to use concurrently. Call sites that have no reporter pass
// nil to the middleware, which then skips reporting entirely.
type Reporter struct {
	lg  *slog.Logger
	dsn string
}

// New builds a Reporter. With an empty (or invalid) DSN only slog receives
// errors — identical to today's behavior — so the app never depends on Sentry
// being reachable or configured.
func New(lg *slog.Logger, dsn, env string) *Reporter {
	r := &Reporter{lg: lg}
	if dsn == "" {
		return r
	}
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      env,
		TracesSampleRate: 0, // error tracking only
	}); err != nil {
		r.lg.Warn("sentry disabled", "error", err.Error())
		return r
	}
	r.dsn = dsn
	r.lg.Info("error tracking enabled", "environment", env)
	return r
}

// Report logs err at error level through the logger and, when Sentry is
// configured, captures it with the same key/value attrs attached as extra data.
func (r *Reporter) Report(ctx context.Context, err error, attrs ...any) {
	r.lg.LogAttrs(ctx, slog.LevelError, "error reported",
		append([]slog.Attr{slog.String("error", err.Error())}, attrsToAttrs(attrs)...)...)
	if r.dsn == "" {
		return
	}
	extra := map[string]any{}
	for i := 0; i+1 < len(attrs); i += 2 {
		if k, ok := attrs[i].(string); ok && k != "" {
			extra[k] = attrs[i+1]
		}
	}
	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetContext("report", sentry.Context(extra))
		sentry.CaptureException(err)
	})
}

// Close flushes any queued Sentry events. Safe to call even when disabled.
func (r *Reporter) Close() {
	if r.dsn != "" {
		sentry.Flush(2 * time.Second)
	}
}

func attrsToAttrs(args []any) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(args)/2)
	for i := 0; i+1 < len(args); i += 2 {
		k, ok := args[i].(string)
		if !ok || k == "" {
			continue
		}
		attrs = append(attrs, slog.Any(k, args[i+1]))
	}
	return attrs
}
