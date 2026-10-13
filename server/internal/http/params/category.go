package params

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
)

// ParseCategoryID parses the optional "category" query param into a *int.
// Absent (or blank) yields (nil, true). Present-but-not-an-integer writes a
// 400 response and yields (nil, false). Surrounding whitespace is trimmed:
// the pins list keeps its own strict inline parse (padded values are 400),
// so this helper serves callers that already trim, like the realtime stream.
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
