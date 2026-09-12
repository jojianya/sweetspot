package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/users"
)

func Me(repo *users.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_id": GetUserID(c),
			"role":    currentRole(repo, c),
		})
	}
}
