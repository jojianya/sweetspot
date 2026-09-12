package auth

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/session"
	"github.com/jojianya/sweetspot247-backend/pkg/jwt"
)

func Logout(bl *session.Blacklist) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := c.Get(CtxJWTClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		jwtClaims := claims.(*jwt.Claims)
		ttl := time.Until(jwtClaims.ExpiresAt.Time)
		if err := bl.Revoke(c.Request.Context(), jwtClaims.ID, ttl); err != nil {
			slog.Default().Error("logout revoke failed", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "logged out"})
	}
}