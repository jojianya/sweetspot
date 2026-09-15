package users

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Get(c *gin.Context) {
	user, err := h.service.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	if viewerID := middleware.GetUserID(c); viewerID != "" && viewerID == user.ID {
		c.JSON(http.StatusOK, user.ToPrivate())
		return
	}
	c.JSON(http.StatusOK, user.ToPublic())
}

func (h *Handler) UpdateRole(c *gin.Context) {
	var req UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.service.UpdateRole(c.Request.Context(), middleware.GetUserID(c), c.Param("id"), req.Role)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		case errors.Is(err, ErrCannotChangeOwnRole), errors.Is(err, ErrCannotDemoteLastOwner):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
	}

	c.JSON(http.StatusOK, user.ToPublic())
}
