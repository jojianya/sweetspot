package response

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

func Errorf(c *gin.Context, status int, err error) {
	c.JSON(status, gin.H{"error": err.Error()})
}

// Internal logs err with msg (plus any extra key/value fields) and responds
// with a 500 carrying a generic message so internals are never leaked to
// clients. Extra fields are passed slog-style: Internal(c, "msg", err, "key", val).
func Internal(c *gin.Context, msg string, err error, fields ...any) {
	if err != nil {
		slog.Error(msg, append([]any{"error", err.Error()}, fields...)...)
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}
