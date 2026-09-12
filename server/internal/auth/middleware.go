package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/session"
	"github.com/jojianya/sweetspot247-backend/internal/users"
	"github.com/jojianya/sweetspot247-backend/pkg/jwt"
)

const (
	CtxUserID    = "user_id"
	CtxRole      = "role"
	CtxJWTClaims = "jwt_claims"
)

func AuthRequired(jwtSecret string, bl *session.Blacklist) gin.HandlerFunc {
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

func OptionalAuth(jwtSecret string, bl *session.Blacklist) gin.HandlerFunc {
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

func currentRole(repo *users.Repository, c *gin.Context) string {
	user, err := repo.GetByID(context.Background(), GetUserID(c))
	if err != nil {
		return ""
	}
	return user.Role
}

func RequireAdmin(repo *users.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := currentRole(repo, c)
		if role != "admin" && role != "owner" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			return
		}
		c.Next()
	}
}

func RequireOwner(repo *users.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := currentRole(repo, c)
		if role != "owner" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "owner access required"})
			return
		}
		c.Next()
	}
}
