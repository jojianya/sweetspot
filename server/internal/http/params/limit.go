package params

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// MaxOffset bounds the "offset" query param on every paginated list.
// Deep offsets make the database walk and discard rows for pages no UI
// ever shows; list endpoints page forward from recent data instead.
const MaxOffset = 10000

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

// ParseOffset parses the "offset" query param (default 0, must be >= 0),
// writing a 400 response and returning ok=false when the value is invalid.
// Offsets above MaxOffset are rejected like over-max limits: deep offsets
// force the database to walk and discard rows for pages no UI ever shows.
func ParseOffset(c *gin.Context) (int, bool) {
	offset := 0
	if oStr := c.Query("offset"); oStr != "" {
		o, err := strconv.Atoi(oStr)
		if err != nil || o < 0 || o > MaxOffset {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("offset must be an integer between 0 and %d", MaxOffset),
			})
			return 0, false
		}
		offset = o
	}
	return offset, true
}
