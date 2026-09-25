package response

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Internal logs err with msg (plus any extra key/value fields) and responds
// with a 500 carrying a generic message so internals are never leaked to
// clients. Extra fields are passed slog-style: Internal(c, "msg", err, "key", val).
// The error is also attached to the gin context so the request logger and the
// ReportErrors middleware can surface the underlying cause in monitoring.
func Internal(c *gin.Context, msg string, err error, fields ...any) {
	if err != nil {
		slog.Error(msg, append([]any{"error", err.Error()}, fields...)...)
		_ = c.Error(err)
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}
