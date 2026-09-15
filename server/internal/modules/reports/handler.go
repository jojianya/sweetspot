package reports

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
)

const (
	reportListDefaultLimit = 50
	reportListMaxLimit     = 200
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pinID := c.Param("id")

	exists, err := h.service.PinExists(c.Request.Context(), pinID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "pin not found"})
		return
	}

	report, err := h.service.CreateReport(c.Request.Context(), pinID, middleware.GetUserID(c), req.Reason)
	if err != nil {
		if errors.Is(err, ErrAlreadyReported) {
			c.JSON(http.StatusConflict, gin.H{"error": "you already reported this pin"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"report": report})
}

func (h *Handler) List(c *gin.Context) {
	var status *string
	if s := c.Query("status"); s != "" {
		switch s {
		case StatusPending, StatusReviewed, StatusActioned:
			status = &s
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "status must be one of pending, reviewed, actioned"})
			return
		}
	}

	limit := reportListDefaultLimit
	if lStr := c.Query("limit"); lStr != "" {
		l, err := strconv.Atoi(lStr)
		if err != nil || l < 1 || l > reportListMaxLimit {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be an integer between 1 and 200"})
			return
		}
		limit = l
	}

	offset := 0
	if oStr := c.Query("offset"); oStr != "" {
		o, err := strconv.Atoi(oStr)
		if err != nil || o < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "offset must be a non-negative integer"})
			return
		}
		offset = o
	}

	reports, err := h.service.ListReports(c.Request.Context(), status, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"reports": reports})
}

func (h *Handler) Review(c *gin.Context) {
	var req ReviewReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	report, err := h.service.ReviewReport(c.Request.Context(), c.Param("id"), req.Action, middleware.GetUserID(c))
	if err != nil {
		switch {
		case errors.Is(err, ErrReportNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
		case errors.Is(err, ErrAlreadyResolved):
			c.JSON(http.StatusConflict, gin.H{"error": "report already resolved"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"report": report})
}
