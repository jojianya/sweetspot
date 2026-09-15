package auth

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/platform/cache"
)

type RouteOptions struct {
	JWTSecret string
	Blacklist *cache.Blacklist
}

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opts RouteOptions) {
	registerLimit := middleware.New(5, time.Minute)
	rg.POST("/auth/register", registerLimit.Middleware(), h.Register)

	loginIPLimit := middleware.New(20, time.Minute)
	rg.POST("/auth/login", loginIPLimit.Middleware(), h.Login)

	rg.POST("/auth/logout", middleware.AuthRequired(opts.JWTSecret, opts.Blacklist), h.Logout)
	rg.GET("/me", middleware.AuthRequired(opts.JWTSecret, opts.Blacklist), h.Me)
}
