package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/users"
	"github.com/jojianya/sweetspot247-backend/pkg/jwt"
)

const (
	CtxUserID = "user_id"
	CtxRole   = "role"
)

func AuthRequired(jwtSecret string) gin.HandlerFunc {
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

		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxRole, claims.Role)
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