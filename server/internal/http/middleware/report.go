package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ReportErrors reports 5xx responses through rep so internal failures surface
// in monitoring even when the handler only wrote a status. Register it after
// RequestLogger; it inspects the final response status after the chain runs.
// When the handler attached an error via c.Error (response.Internal does), that
// error is reported; otherwise the status alone is.
func ReportErrors(rep ErrorReporter) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if rep == nil || c.Writer.Status() < http.StatusInternalServerError {
			return
		}
		var err error
		if len(c.Errors) > 0 {
			err = c.Errors.Last().Err
		} else {
			err = fmt.Errorf("request failed with status %d", c.Writer.Status())
		}
		rep.Report(c.Request.Context(), err,
			"request_id", c.GetHeader("X-Request-ID"),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"ip", c.ClientIP(),
		)
	}
}
