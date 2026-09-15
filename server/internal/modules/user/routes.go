package users

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/platform/cache"
	"github.com/jojianya/sweetspot247-backend/pkg/validid"
)

type RouteOptions struct {
	JWTSecret string
	Blacklist *cache.Blacklist
}

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opts RouteOptions) {
	rg.GET("/users/:id", validid.Middleware(), h.Get)
	rg.PATCH("/users/:id/role",
		middleware.AuthRequired(opts.JWTSecret, opts.Blacklist),
		RequireOwner(h.service),
		validid.Middleware(),
		h.UpdateRole,
	)
}

// CurrentRole resolves the caller role from the request context using the user service.
func CurrentRole(svc Service, c *gin.Context) string {
	user, err := svc.GetByID(context.Background(), middleware.GetUserID(c))
	if err != nil {
		return ""
	}
	return user.Role
}

func RequireAdmin(svc Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if role := CurrentRole(svc, c); role != RoleAdmin && role != RoleOwner {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			return
		}
		c.Next()
	}
}

func RequireOwner(svc Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if role := CurrentRole(svc, c); role != RoleOwner {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "owner access required"})
			return
		}
		c.Next()
	}
}
