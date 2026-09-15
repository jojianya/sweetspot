package reports

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	reportListDefaultLimit = 50
	reportListMaxLimit     = 200
)

func ListReports(repo *Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
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

		reports, err := repo.ListReports(c.Request.Context(), status, limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"reports": reports})
	}
}
