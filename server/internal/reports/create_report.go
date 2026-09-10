package reports

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jojianya/sweetspot247-backend/internal/auth"
)

func CreateReport(repo *Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateReportRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		pinID := c.Param("id")

		exists, err := repo.PinExists(c.Request.Context(), pinID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "pin not found"})
			return
		}

		report, err := repo.CreateReport(c.Request.Context(), pinID, auth.GetUserID(c), req.Reason)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				c.JSON(http.StatusConflict, gin.H{"error": "you already reported this pin"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"report": report})
	}
}
