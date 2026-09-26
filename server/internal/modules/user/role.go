package users

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
)

// RoleReader is the subset of Service needed to resolve a caller's role.
//
// Narrower than Service on purpose: every module that needs a role check
// depends on this single-method interface instead of the full user service,
// which keeps the dependency obvious and lets tests supply a one-method stub
// rather than implementing all ten.
type RoleReader interface {
	GetByID(ctx context.Context, id string) (User, error)
}

// CurrentRole resolves the caller role from the request context using the user
// service.
//
// The role is read live from the database rather than taken from the JWT. A
// role claim baked in at login goes stale: demoting an admin would otherwise
// leave them authorized until their token expired.
func CurrentRole(svc RoleReader, c *gin.Context) string {
	userID := middleware.GetUserID(c)
	if userID == "" {
		return ""
	}
	user, err := svc.GetByID(c.Request.Context(), userID)
	if err != nil {
		return ""
	}
	return user.Role
}

// IsModerator reports whether the caller currently holds a moderation role.
//
// Like CurrentRole this reads through to the database, so a demotion takes
// effect on the caller's next request.
func IsModerator(svc RoleReader, c *gin.Context) bool {
	role := CurrentRole(svc, c)
	return role == RoleAdmin || role == RoleOwner
}

func RequireAdmin(svc RoleReader) gin.HandlerFunc {
	return func(c *gin.Context) {
		if role := CurrentRole(svc, c); role != RoleAdmin && role != RoleOwner {
			response.AbortError(c, http.StatusForbidden, "admin access required")
			return
		}
		c.Next()
	}
}

func RequireOwner(svc RoleReader) gin.HandlerFunc {
	return func(c *gin.Context) {
		if role := CurrentRole(svc, c); role != RoleOwner {
			response.AbortError(c, http.StatusForbidden, "owner access required")
			return
		}
		c.Next()
	}
}
