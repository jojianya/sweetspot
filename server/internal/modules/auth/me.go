package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
)

func Me(svc users.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_id": middleware.GetUserID(c),
			"role":    users.CurrentRole(svc, c),
		})
	}
}
