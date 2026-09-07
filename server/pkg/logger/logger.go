package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lmittmann/tint"
)

const (
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiRed    = "\x1b[31m"
	ansiReset  = "\x1b[0m"
)

const (
	FormatText = "text"
	FormatJSON = "json"
)

type ctxKey string

const loggerKey ctxKey = "logger"

// defaultLogger is the package fallback used when no request-scoped logger is
// attached. SetDefault and Init keep it in sync with process-level defaults.
var defaultLogger = slog.Default()

// colorStatus controls whether Status() attributes rendered by the request
// logging middleware are wrapped in ANSI colors. It is enabled for text output
// and disabled for JSON so production logs stay machine-parsable.
var colorStatus bool

// Init returns a log/slog logger configured with the given level and format.
// Level is one of: debug, info, warn, error. Format is one of: text, json.
// It defaults to info/text if an unrecognized value is supplied.
func Init(level, format string) *slog.Logger {
	lvl := slog.LevelInfo
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: lvl}

	var handler slog.Handler
	switch strings.ToLower(format) {
	case FormatJSON:
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = tint.NewHandler(os.Stdout, &tint.Options{
			Level:      lvl,
			TimeFormat: time.Kitchen,
			AddSource:  false,
			ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
				if a.Key == slog.LevelKey {
					level := a.Value.Any().(slog.Level)
					var badge string
					switch {
					case level >= slog.LevelError:
						badge = ansiRed + ansiBold + "ERR" + ansiReset
					case level >= slog.LevelWarn:
						badge = ansiYellow + ansiBold + "WRN" + ansiReset
					default:
						badge = ansiGreen + ansiBold + "INF" + ansiReset
					}
					return slog.String(slog.LevelKey, badge)
				}
				return a
			},
		})
	}

	colorStatus = strings.ToLower(format) != FormatJSON

	return slog.New(handler)
}

// SetDefault makes l the default logger used by package slog throughout the
// process and the fallback used by FromContext.
func SetDefault(l *slog.Logger) {
	slog.SetDefault(l)
	defaultLogger = l
}

// FromContext returns the request-scoped logger attached to the gin context.
// If none was attached it falls back to the package default logger, so
// handlers can safely use it even without the logging middleware.
func FromContext(c *gin.Context) *slog.Logger {
	if c == nil {
		return defaultLogger
	}
	if l, ok := c.Get(string(loggerKey)); ok {
		if lg, ok := l.(*slog.Logger); ok {
			return lg
		}
	}
	return defaultLogger
}

// WithLogger returns a context carrying the given logger.
func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}
