package users

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/http/validid"
	"github.com/jojianya/sweetspot247-backend/internal/platform/cache"
)

type RouteOptions struct {
	JWTSecret string
	Blacklist *cache.Blacklist
	Sessions  middleware.SessionChecker
}

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opts RouteOptions) {
	rg.GET("/users",
		middleware.AuthRequired(opts.JWTSecret, opts.Blacklist, opts.Sessions),
		RequireOwner(h.service),
		h.List,
	)
	rg.GET("/users/:id", middleware.OptionalAuth(opts.JWTSecret, opts.Blacklist, opts.Sessions), validid.Middleware(), h.Get)
	// Avatar uploads share the createLimit budget shape (10/min per IP) but
	// use a dedicated instance so avatar and pin PATCH floods do not share
	// budget. Before auth, like POST /pins, so unauthenticated floods count.
	avatarPatchLimit := middleware.New(10, time.Minute)
	rg.PATCH("/users/me",
		avatarPatchLimit.Middleware(),
		middleware.AuthRequired(opts.JWTSecret, opts.Blacklist, opts.Sessions),
		h.UpdateMe,
	)
	rg.PATCH("/users/:id/role",
		middleware.AuthRequired(opts.JWTSecret, opts.Blacklist, opts.Sessions),
		RequireOwner(h.service),
		validid.Middleware(),
		h.UpdateRole,
	)
}

// CurrentRole, IsModerator, RequireAdmin and RequireOwner live in role.go.
