package middleware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorReporter is the subset of *report.Reporter that panics and 5xx
// responses are reported through. Defined here so tests can substitute a stub
// without importing the observability package.
type ErrorReporter interface {
	Report(ctx context.Context, err error, attrs ...any)
}

// Recover turns panics into 500s and, when rep is non-nil, reports the panic
// with request context attached so it lands in monitoring (slog + Sentry).
func Recover(rep ErrorReporter) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rcv := recover(); rcv != nil {
				err, ok := rcv.(error)
				if !ok {
					err = fmt.Errorf("panic: %v", rcv)
				}
				if rep != nil {
					rep.Report(c.Request.Context(), err,
						"request_id", c.GetHeader("X-Request-ID"),
						"method", c.Request.Method,
						"path", c.Request.URL.Path,
						"recovered", true,
					)
				}
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}
