package reactions

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
)

type Handler struct {
	repo Repository
	svc  *Service
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo, svc: NewService(repo)}
}

// service returns the service, building it from the handler's repository when
// the handler was constructed as a struct literal (as some tests do) instead
// of via NewHandler. Same convention as favorites and social.
func (h *Handler) service() *Service {
	if h.svc != nil {
		return h.svc
	}
	return NewService(h.repo)
}

// React is PUT /pins/:id/good-spot.
//
// It answers 200 with the caller's state after the write rather than 204, so a
// client never has to re-read the pin to learn the new count — and so a
// rejected tap is distinguishable from an accepted one without a second call.
func (h *Handler) React(c *gin.Context) {
	pinID := c.Param("id")
	result, err := h.service().React(c.Request.Context(), middleware.GetUserID(c), pinID)
	if err != nil {
		respondError(c, err, "reactions: react", pinID)
		return
	}
	response.OK(c, result)
}

// Unreact is DELETE /pins/:id/good-spot. Removing a reaction that was never
// there still answers 200 with reacted:false, because the end state is the one
// the request asked for.
func (h *Handler) Unreact(c *gin.Context) {
	pinID := c.Param("id")
	result, err := h.service().Unreact(c.Request.Context(), middleware.GetUserID(c), pinID)
	if err != nil {
		respondError(c, err, "reactions: unreact", pinID)
		return
	}
	response.OK(c, result)
}

// respondError maps the service's errors onto status codes. Anything that is
// not a known business rule is a 500, so a repository failure is never reported
// as a 404 or a 403.
func respondError(c *gin.Context, err error, logMsg, pinID string) {
	switch {
	case errors.Is(err, ErrNotFound):
		// Deliberately identical for hidden and missing: the caller cannot tell
		// a moderated pin from a nonexistent one.
		response.NotFound(c, "pin not found")
	case errors.Is(err, ErrOwnPin):
		response.Forbidden(c, err.Error())
	default:
		response.Internal(c, logMsg, err, "pin_id", pinID)
	}
}
