package collections

import (
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
)

// normalize validates and trims the name/description fields.
func normalize(c *gin.Context, name string, description *string) (string, *string, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		response.BadRequest(c, "name is required")
		return "", nil, false
	}
	if utf8.RuneCountInString(name) > maxCollectionNameLength {
		response.BadRequest(c, "name must be at most 60 characters")
		return "", nil, false
	}
	if description != nil {
		trimmed := strings.TrimSpace(*description)
		if utf8.RuneCountInString(trimmed) > maxCollectionDescLength {
			response.BadRequest(c, "description must be at most 200 characters")
			return "", nil, false
		}
		description = &trimmed
	}
	return name, description, true
}
