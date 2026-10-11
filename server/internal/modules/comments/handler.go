package comments

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	httpx "github.com/jojianya/sweetspot247-backend/internal/http/params"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
)

const (
	maxCommentLength = 500

	commentListDefaultLimit = 50
	commentListMaxLimit     = 200
)

type Handler struct {
	repo Repository
	// roles resolves moderation rights from the database rather than the JWT,
	// so a demotion takes effect on the caller's next request.
	roles users.RoleReader
	svc   *Service
}

func NewHandler(repo Repository, roles users.RoleReader) *Handler {
	return &Handler{repo: repo, roles: roles, svc: NewService(repo)}
}

// service returns the service, building it from the handler's repository when
// the handler was constructed as a struct literal (as some tests do) instead
// of via NewHandler.
func (h *Handler) service() *Service {
	if h.svc != nil {
		return h.svc
	}
	return NewService(h.repo)
}

// isModerator reports whether the caller holds a moderation role.
func (h *Handler) isModerator(c *gin.Context) bool {
	return users.IsModerator(h.roles, c)
}

func (h *Handler) List(c *gin.Context) {
	limit, ok := httpx.ParseLimit(c, commentListDefaultLimit, commentListMaxLimit)
	if !ok {
		return
	}
	offset, ok := httpx.ParseOffset(c)
	if !ok {
		return
	}

	comments, total, err := h.service().ListByPin(c.Request.Context(), c.Param("id"), limit, offset)
	if err != nil {
		if errors.Is(err, ErrPinNotFound) {
			response.NotFound(c, "pin not found")
			return
		}
		response.Internal(c, "comments: list", err, "pin_id", c.Param("id"))
		return
	}
	response.OK(c, gin.H{"comments": comments, "total": total})
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "body is required")
		return
	}

	comment, err := h.service().Create(c.Request.Context(), c.Param("id"), middleware.GetUserID(c), req.Body)
	if err != nil {
		switch {
		case errors.Is(err, errEmptyBody):
			response.BadRequest(c, "body is required")
		case errors.Is(err, errBodyTooLong):
			response.BadRequest(c, "body must be at most 500 characters")
		case errors.Is(err, ErrPinNotFound):
			response.NotFound(c, "pin not found")
		default:
			response.Internal(c, "comments: create", err, "pin_id", c.Param("id"))
		}
		return
	}

	response.Created(c, gin.H{"comment": comment})
}

func (h *Handler) Delete(c *gin.Context) {
	if err := h.service().Delete(c.Request.Context(), c.Param("id"), middleware.GetUserID(c), h.isModerator(c)); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(c, "comment not found")
		case errors.Is(err, ErrForbidden):
			response.Forbidden(c, "you can only delete your own comments")
		default:
			response.Internal(c, "comments: delete", err, "comment_id", c.Param("id"))
		}
		return
	}

	response.NoContent(c)
}
