package collections

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
)

// requireOwner aborts unless the caller owns the collection (or moderates),
// returning the collection so callers can reuse the read.
func (h *Handler) requireOwner(c *gin.Context) (Collection, bool) {
	collection, err := h.service().RequireOwner(c.Request.Context(), c.Param("id"), middleware.GetUserID(c), users.IsModeratorHandler(h.roles, c))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "collection not found")
			return Collection{}, false
		}
		if errors.Is(err, ErrForbidden) {
			response.Forbidden(c, "you can only modify your own collections")
			return Collection{}, false
		}
		response.Internal(c, "collections: get", err, "collection_id", c.Param("id"))
		return Collection{}, false
	}
	return collection, true
}
