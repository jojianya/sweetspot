package favorites

import (
	"log/slog"
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

func (h *Handler) Save(c *gin.Context) {
	userID := middleware.GetUserID(c)
	pinID := c.Param("id")

	exists, err := h.service.PinExists(c.Request.Context(), pinID)
	if err != nil {
		slog.Error("favorite: pin exists", "pin_id", pinID, "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "pin not found"})
		return
	}

	if err := h.service.SavePin(c.Request.Context(), userID, pinID); err != nil {
		slog.Error("favorite: save", "user_id", userID, "pin_id", pinID, "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) Unsave(c *gin.Context) {
	userID := middleware.GetUserID(c)
	pinID := c.Param("id")

	saved, err := h.service.IsSaved(c.Request.Context(), userID, pinID)
	if err != nil {
		slog.Error("favorite: is saved", "user_id", userID, "pin_id", pinID, "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if !saved {
		c.JSON(http.StatusNotFound, gin.H{"error": "favorite not found"})
		return
	}

	if err := h.service.UnsavePin(c.Request.Context(), userID, pinID); err != nil {
		slog.Error("favorite: unsave", "user_id", userID, "pin_id", pinID, "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) GetSaved(c *gin.Context) {
	userID := middleware.GetUserID(c)

	entries, err := h.service.List(c.Request.Context(), userID)
	if err != nil {
		slog.Error("favorite: list", "user_id", userID, "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"pins": entries})
}

func (h *Handler) GetSavedIDs(c *gin.Context) {
	userID := middleware.GetUserID(c)

	ids, err := h.service.ListIDs(c.Request.Context(), userID)
	if err != nil {
		slog.Error("favorite: list ids", "user_id", userID, "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ids": ids})
}