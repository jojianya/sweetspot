package reports

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/auth"
)

func ReviewReport(repo *Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ReviewReportRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		report, err := repo.ReviewReport(c.Request.Context(), c.Param("id"), req.Action, auth.GetUserID(c))
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
}
