package users

import (
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
	rg.GET("/users",
		middleware.AuthRequired(opts.JWTSecret, opts.Blacklist),
		RequireOwner(h.service),
		h.List,
	)
	rg.GET("/users/:id", middleware.OptionalAuth(opts.JWTSecret, opts.Blacklist), validid.Middleware(), h.Get)
	rg.PATCH("/users/me",
		middleware.AuthRequired(opts.JWTSecret, opts.Blacklist),
		h.UpdateMe,
	)
	rg.PATCH("/users/:id/role",
		middleware.AuthRequired(opts.JWTSecret, opts.Blacklist),
		RequireOwner(h.service),
		validid.Middleware(),
		h.UpdateRole,
	)
}

// CurrentRole, IsModerator, RequireAdmin and RequireOwner live in role.go.
