package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Me() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_id": GetUserID(c),
			"role":    GetRole(c),
		})
	}
}
