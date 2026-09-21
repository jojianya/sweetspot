package social

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
	rg.GET("/users/:id/stats", validid.Middleware(), middleware.OptionalAuth(opts.JWTSecret, opts.Blacklist), h.Stats)
	rg.PUT("/users/:id/follow", validid.Middleware(), middleware.AuthRequired(opts.JWTSecret, opts.Blacklist), h.Follow)
	rg.DELETE("/users/:id/follow", validid.Middleware(), middleware.AuthRequired(opts.JWTSecret, opts.Blacklist), h.Unfollow)
	rg.GET("/feed", middleware.AuthRequired(opts.JWTSecret, opts.Blacklist), h.Feed)
}
