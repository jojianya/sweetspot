package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/platform/cache"
	"github.com/jojianya/sweetspot247-backend/pkg/jwt"
)

const (
	CtxUserID    = "user_id"
	CtxRole      = "role"
	CtxJWTClaims = "jwt_claims"
)

func AuthRequired(jwtSecret string, bl *cache.Blacklist) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid Authorization header"})
			return
		}

		tokenString := strings.TrimPrefix(header, "Bearer ")
		claims, err := jwt.Validate(jwtSecret, tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		if bl != nil {
			revoked, err := bl.IsRevoked(c.Request.Context(), claims.ID)
			if err != nil {
				slog.Default().Warn("blacklist check failed", "error", err.Error())
			} else if revoked {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session was logged out, please sign in again"})
				return
			}
		}

		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxRole, claims.Role)
		c.Set(CtxJWTClaims, claims)
		c.Next()
	}
}

func OptionalAuth(jwtSecret string, bl *cache.Blacklist) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.Next()
			return
		}

		tokenString := strings.TrimPrefix(header, "Bearer ")
		claims, err := jwt.Validate(jwtSecret, tokenString)
		if err != nil {
			c.Next()
			return
		}

		if bl != nil {
			revoked, err := bl.IsRevoked(c.Request.Context(), claims.ID)
			if err != nil {
				slog.Default().Warn("blacklist check failed", "error", err.Error())
			} else if revoked {
				c.Next()
				return
			}
		}

		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxRole, claims.Role)
		c.Set(CtxJWTClaims, claims)
		c.Next()
	}
}

func GetUserID(c *gin.Context) string {
	return c.GetString(CtxUserID)
}

func GetRole(c *gin.Context) string {
	return c.GetString(CtxRole)
}
