package validid

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const paramName = "id"

func IsUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < 36; i++ {
		b := s[i]
		switch i {
		case 8, 13, 18, 23:
			if b != '-' {
				return false
			}
		default:
			if !isHex(b) {
				return false
			}
		}
	}
	return true
}

func isHex(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !IsUUID(c.Param(paramName)) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		c.Next()
	}
}
