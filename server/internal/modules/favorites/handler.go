package favorites

import (
	"errors"

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

func (h *Handler) Save(c *gin.Context) {
	userID := middleware.GetUserID(c)
	pinID := c.Param("id")

	if err := h.service().Save(c.Request.Context(), userID, pinID); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "pin not found")
			return
		}
		response.Internal(c, "favorite: save", err, "user_id", userID, "pin_id", pinID)
		return
	}

	response.NoContent(c)
}

func (h *Handler) Unsave(c *gin.Context) {
	userID := middleware.GetUserID(c)
	pinID := c.Param("id")

	if err := h.service().Unsave(c.Request.Context(), userID, pinID); err != nil {
		if errors.Is(err, ErrFavoriteNotFound) {
			response.NotFound(c, "favorite not found")
			return
		}
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

	entries, total, err := h.service().List(c.Request.Context(), userID, limit, offset)
	if err != nil {
		response.Internal(c, "favorite: list", err, "user_id", userID)
		return
	}

	response.OK(c, gin.H{"pins": entries, "total": total})
}

func (h *Handler) GetSavedIDs(c *gin.Context) {
	userID := middleware.GetUserID(c)

	ids, err := h.service().ListIDs(c.Request.Context(), userID)
	if err != nil {
		response.Internal(c, "favorite: list ids", err, "user_id", userID)
		return
	}

	response.OK(c, gin.H{"ids": ids})
}
