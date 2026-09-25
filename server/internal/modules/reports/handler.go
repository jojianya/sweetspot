package reports

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	httpx "github.com/jojianya/sweetspot247-backend/internal/http/params"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
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
		response.BadRequest(c, err.Error())
		return
	}

	pinID := c.Param("id")

	exists, err := h.service.PinExists(c.Request.Context(), pinID)
	if err != nil {
		response.Internal(c, "report: pin exists", err, "pin_id", pinID)
		return
	}
	if !exists {
		response.NotFound(c, "pin not found")
		return
	}

	report, err := h.service.CreateReport(c.Request.Context(), pinID, middleware.GetUserID(c), req.Reason)
	if err != nil {
		if errors.Is(err, ErrAlreadyReported) {
			response.Conflict(c, "you already reported this pin")
			return
		}
		response.Internal(c, "report: create", err, "pin_id", pinID)
		return
	}

	response.Created(c, gin.H{"report": report})
}

func (h *Handler) List(c *gin.Context) {
	var status *string
	if s := c.Query("status"); s != "" {
		switch s {
		case StatusPending, StatusReviewed, StatusActioned:
			status = &s
		default:
			response.BadRequest(c, "status must be one of pending, reviewed, actioned")
			return
		}
	}

	limit, ok := httpx.ParseLimit(c, reportListDefaultLimit, reportListMaxLimit)
	if !ok {
		return
	}

	offset := 0
	if oStr := c.Query("offset"); oStr != "" {
		o, err := strconv.Atoi(oStr)
		if err != nil || o < 0 {
			response.BadRequest(c, "offset must be a non-negative integer")
			return
		}
		offset = o
	}

	reports, err := h.service.ListReports(c.Request.Context(), status, limit, offset)
	if err != nil {
		response.Internal(c, "report: list", err)
		return
	}

	response.OK(c, gin.H{"reports": reports})
}

func (h *Handler) Review(c *gin.Context) {
	var req ReviewReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	report, err := h.service.ReviewReport(c.Request.Context(), c.Param("id"), req.Action, middleware.GetUserID(c))
	if err != nil {
		switch {
		case errors.Is(err, ErrReportNotFound):
			response.NotFound(c, "report not found")
		case errors.Is(err, ErrAlreadyResolved):
			response.Conflict(c, "report already resolved")
		default:
			response.Internal(c, "report: review", err)
		}
		return
	}

	response.OK(c, gin.H{"report": report})
}
