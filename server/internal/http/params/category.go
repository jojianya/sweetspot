package params

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
)

// ParseCategoryID parses the optional "category" query param into a *int.
// Absent (or blank) yields (nil, true). Present-but-not-an-integer writes a
// 400 response and yields (nil, false).
func ParseCategoryID(c *gin.Context) (*int, bool) {
	raw := strings.TrimSpace(c.Query("category"))
	if raw == "" {
		return nil, true
	}
	id, err := strconv.Atoi(raw)
	if err != nil {
		response.BadRequest(c, "category must be an integer")
		return nil, false
	}
	return &id, true
}
