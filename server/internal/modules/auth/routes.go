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
	registerIP := middleware.New(5, time.Minute)
	registerGlobal := middleware.New(20, time.Minute)

	loginIP := middleware.New(20, time.Minute)
	loginGlobal := middleware.New(60, time.Minute)

	rg.POST("/auth/register",
		registerGlobal.MiddlewareKeyed(globalKey("register")),
		registerIP.Middleware(),
		h.Register)

	rg.POST("/auth/login",
		loginGlobal.MiddlewareKeyed(globalKey("login")),
		loginIP.Middleware(),
		h.Login)

	rg.POST("/auth/logout", middleware.AuthRequired(opts.JWTSecret, opts.Blacklist), h.Logout)
	rg.GET("/me", middleware.AuthRequired(opts.JWTSecret, opts.Blacklist), h.Me)
}

// globalKey is a constant-function key so the MiddlewareKeyed limiter applies a
// single counter across all clients rather than per source IP.
func globalKey(name string) func(*gin.Context) string {
	return func(*gin.Context) string { return name }
}
