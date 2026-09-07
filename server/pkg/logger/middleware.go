package logger

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogger returns a Gin middleware that logs one line per completed
// request through slog:
//
//   - attaches a request-scoped logger (with request_id) to the gin context
//     so handlers can call logger.FromContext(c)
//   - logs method, path, status, latency and client ip
//   - uses slog.Error for 5xx, slog.Warn for 4xx and slog.Info for the rest so
//     failures stand out (tint colors these by level)
//   - colorizes the status code itself (green/yellow/red) in text output
//   - skips logging successful requests for the given skip paths (e.g. health
//     checks), but still logs them if they return a 4xx/5xx status
func RequestLogger(base *slog.Logger, skipPaths ...string) gin.HandlerFunc {
	skip := make(map[string]struct{}, len(skipPaths))
	for _, p := range skipPaths {
		skip[p] = struct{}{}
	}

	return func(c *gin.Context) {
		start := time.Now()

		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = genRequestID()
		}
		c.Header("X-Request-ID", requestID)

		reqLogger := base.With(
			slog.String("request_id", requestID),
		)
		c.Set(string(loggerKey), reqLogger)

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		// if _, ok := skip[c.Request.URL.Path]; ok && status < 400 {
		// 	return
		// }

		attrs := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			statusAttr(status),
			slog.String("latency", fmtLatency(latency)),
			slog.String("ip", c.ClientIP()),
		}

		switch {
		case status >= 500:
			var errStr string
			if len(c.Errors) > 0 {
				errStr = c.Errors.Last().Error()
			} else {
				errStr = fmt.Sprintf("status %d", status)
			}
			attrs = append(attrs, slog.String("err", errStr))
			reqLogger.LogAttrs(c.Request.Context(), slog.LevelError, "request failed", attrs...)
		case status >= 400:
			reqLogger.LogAttrs(c.Request.Context(), slog.LevelWarn, "request", attrs...)
		default:
			reqLogger.LogAttrs(c.Request.Context(), slog.LevelInfo, "request", attrs...)
		}
	}
}

// Middleware is kept as an alias for RequestLogger to ease the transition for
// existing callers.
var Middleware = RequestLogger

// statusAttr renders the HTTP status code, wrapped in ANSI color when color
// output is enabled and as a plain int in JSON mode.
func statusAttr(code int) slog.Attr {
	if colorStatus {
		return slog.String("status", colorizeStatus(code))
	}
	return slog.Int("status", code)
}

// colorizeStatus maps a status code to a color: green 2xx, yellow 4xx, red 5xx.
func colorizeStatus(code int) string {
	var color string
	switch {
	case code >= 500:
		color = ansiRed
	case code >= 400:
		color = ansiYellow
	default:
		color = ansiGreen
	}
	return color + strconv.Itoa(code) + ansiReset
}

// fmtLatency formats a duration as a short human-readable string (e.g. 8.2ms, 1.1s).
func fmtLatency(d time.Duration) string {
	switch {
	case d < time.Millisecond:
		return fmt.Sprintf("%.0fµs", float64(d)/float64(time.Microsecond))
	case d < time.Second:
		return fmt.Sprintf("%.1fms", float64(d)/float64(time.Millisecond))
	default:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}

// genRequestID returns a random hex string used as a request identifier. It
// falls back to the current time in seconds if the random source fails.
func genRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return strings.ReplaceAll(time.Now().Format("150405.000000000"), ".", "")
	}
	return hex.EncodeToString(b)
}