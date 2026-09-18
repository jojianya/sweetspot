package params

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ParseLimit parses the "limit" query param against a default and max,
// writing a 400 response and returning ok=false when the value is not an
// integer or is outside [1, max].
func ParseLimit(c *gin.Context, def, max int) (int, bool) {
	limit := def
	if lStr := c.Query("limit"); lStr != "" {
		l, err := strconv.Atoi(lStr)
		if err != nil || l < 1 || l > max {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("limit must be an integer between 1 and %d", max),
			})
			return 0, false
		}
		limit = l
	}
	return limit, true
}
