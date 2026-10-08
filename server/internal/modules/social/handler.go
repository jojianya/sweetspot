package social

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	httpx "github.com/jojianya/sweetspot247-backend/internal/http/params"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
)

const feedDefaultLimit = 50
const feedMaxLimit = 100

type Handler struct {
	repo Repository
	svc  *Service
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo, svc: NewService(repo)}
}

// service returns the service, building it from the handler's repository
// when the handler was constructed as a struct literal (as some tests do)
// instead of via NewHandler.
func (h *Handler) service() *Service {
	if h.svc != nil {
		return h.svc
	}
	return NewService(h.repo)
}

func (h *Handler) followTarget(c *gin.Context) (string, bool) {
	targetID, err := h.service().FollowTarget(c.Request.Context(), middleware.GetUserID(c), c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, ErrSelfFollow):
			response.BadRequest(c, "you cannot follow yourself")
		case errors.Is(err, ErrNotFound):
			response.NotFound(c, "user not found")
		default:
			response.Internal(c, "social: user exists", err, "user_id", c.Param("id"))
		}
		return "", false
	}
	return targetID, true
}

func (h *Handler) Follow(c *gin.Context) {
	targetID, ok := h.followTarget(c)
	if !ok {
		return
	}

	if err := h.service().Follow(c.Request.Context(), middleware.GetUserID(c), targetID); err != nil {
		response.Internal(c, "social: follow", err, "user_id", middleware.GetUserID(c), "target", targetID)
		return
	}

	response.NoContent(c)
}

func (h *Handler) Unfollow(c *gin.Context) {
	targetID, ok := h.followTarget(c)
	if !ok {
		return
	}

	if err := h.service().Unfollow(c.Request.Context(), middleware.GetUserID(c), targetID); err != nil {
		response.Internal(c, "social: unfollow", err, "user_id", middleware.GetUserID(c), "target", targetID)
		return
	}

	response.NoContent(c)
}

func (h *Handler) Stats(c *gin.Context) {
	userID := c.Param("id")

	stats, err := h.service().UserStats(c.Request.Context(), middleware.GetUserID(c), userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "user not found")
			return
		}
		var fail *statsFailure
		if errors.As(err, &fail) {
			response.Internal(c, fail.log, fail.err, fail.args...)
			return
		}
		response.Internal(c, "social: stats", err, "user_id", userID)
		return
	}

	response.OK(c, stats)
}

func (h *Handler) Feed(c *gin.Context) {
	limit, ok := httpx.ParseLimit(c, feedDefaultLimit, feedMaxLimit)
	if !ok {
		return
	}

	pins, err := h.service().Feed(c.Request.Context(), middleware.GetUserID(c), limit)
	if err != nil {
		response.Internal(c, "social: feed", err, "user_id", middleware.GetUserID(c))
		return
	}

	response.OK(c, gin.H{"pins": pins})
}
