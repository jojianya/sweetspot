package social

import (
	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	httpx "github.com/jojianya/sweetspot247-backend/internal/http/params"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
)

const feedDefaultLimit = 50
const feedMaxLimit = 100

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) followTarget(c *gin.Context) (string, bool) {
	targetID := c.Param("id")
	if targetID == middleware.GetUserID(c) {
		response.BadRequest(c, "you cannot follow yourself")
		return "", false
	}

	exists, err := h.repo.UserExists(c.Request.Context(), targetID)
	if err != nil {
		response.Internal(c, "social: user exists", err, "user_id", targetID)
		return "", false
	}
	if !exists {
		response.NotFound(c, "user not found")
		return "", false
	}
	return targetID, true
}

func (h *Handler) Follow(c *gin.Context) {
	targetID, ok := h.followTarget(c)
	if !ok {
		return
	}

	if err := h.repo.Follow(c.Request.Context(), middleware.GetUserID(c), targetID); err != nil {
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

	if err := h.repo.Unfollow(c.Request.Context(), middleware.GetUserID(c), targetID); err != nil {
		response.Internal(c, "social: unfollow", err, "user_id", middleware.GetUserID(c), "target", targetID)
		return
	}

	response.NoContent(c)
}

func (h *Handler) Stats(c *gin.Context) {
	userID := c.Param("id")

	exists, err := h.repo.UserExists(c.Request.Context(), userID)
	if err != nil {
		response.Internal(c, "social: user exists", err, "user_id", userID)
		return
	}
	if !exists {
		response.NotFound(c, "user not found")
		return
	}

	viewerID := middleware.GetUserID(c)
	stats := Stats{}
	if stats.Followers, err = h.repo.CountFollowers(c.Request.Context(), userID); err != nil {
		response.Internal(c, "social: count followers", err, "user_id", userID)
		return
	}
	if stats.Following, err = h.repo.CountFollowing(c.Request.Context(), userID); err != nil {
		response.Internal(c, "social: count following", err, "user_id", userID)
		return
	}
	if stats.PinsCount, err = h.repo.CountPins(c.Request.Context(), userID); err != nil {
		response.Internal(c, "social: count pins", err, "user_id", userID)
		return
	}
	if viewerID != "" && viewerID != userID {
		if stats.IsFollowing, err = h.repo.IsFollowing(c.Request.Context(), viewerID, userID); err != nil {
			response.Internal(c, "social: is following", err, "user_id", viewerID, "target", userID)
			return
		}
	}

	response.OK(c, stats)
}

func (h *Handler) Feed(c *gin.Context) {
	limit, ok := httpx.ParseLimit(c, feedDefaultLimit, feedMaxLimit)
	if !ok {
		return
	}

	pins, err := h.repo.Feed(c.Request.Context(), middleware.GetUserID(c), limit)
	if err != nil {
		response.Internal(c, "social: feed", err, "user_id", middleware.GetUserID(c))
		return
	}

	response.OK(c, gin.H{"pins": pins})
}
