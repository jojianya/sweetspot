package favorites

import (
	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	httpx "github.com/jojianya/sweetspot247-backend/internal/http/params"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
)

const (
	savedListDefaultLimit = 50
	savedListMaxLimit     = 200
)

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Save(c *gin.Context) {
	userID := middleware.GetUserID(c)
	pinID := c.Param("id")

	exists, err := h.repo.PinExists(c.Request.Context(), pinID)
	if err != nil {
		response.Internal(c, "favorite: pin exists", err, "pin_id", pinID)
		return
	}
	if !exists {
		response.NotFound(c, "pin not found")
		return
	}

	if err := h.repo.Save(c.Request.Context(), userID, pinID); err != nil {
		response.Internal(c, "favorite: save", err, "user_id", userID, "pin_id", pinID)
		return
	}

	response.NoContent(c)
}

func (h *Handler) Unsave(c *gin.Context) {
	userID := middleware.GetUserID(c)
	pinID := c.Param("id")

	saved, err := h.repo.IsSaved(c.Request.Context(), userID, pinID)
	if err != nil {
		response.Internal(c, "favorite: is saved", err, "user_id", userID, "pin_id", pinID)
		return
	}
	if !saved {
		response.NotFound(c, "favorite not found")
		return
	}

	if err := h.repo.Unsave(c.Request.Context(), userID, pinID); err != nil {
		response.Internal(c, "favorite: unsave", err, "user_id", userID, "pin_id", pinID)
		return
	}

	response.NoContent(c)
}

func (h *Handler) GetSaved(c *gin.Context) {
	userID := middleware.GetUserID(c)

	limit, ok := httpx.ParseLimit(c, savedListDefaultLimit, savedListMaxLimit)
	if !ok {
		return
	}
	offset, ok := httpx.ParseOffset(c)
	if !ok {
		return
	}

	entries, total, err := h.repo.List(c.Request.Context(), userID, limit, offset)
	if err != nil {
		response.Internal(c, "favorite: list", err, "user_id", userID)
		return
	}

	response.OK(c, gin.H{"pins": entries, "total": total})
}

func (h *Handler) GetSavedIDs(c *gin.Context) {
	userID := middleware.GetUserID(c)

	ids, err := h.repo.ListIDs(c.Request.Context(), userID)
	if err != nil {
		response.Internal(c, "favorite: list ids", err, "user_id", userID)
		return
	}

	response.OK(c, gin.H{"ids": ids})
}
