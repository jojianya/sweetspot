package auth

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/internal/platform/cache"
)

type RouteOptions struct {
	JWTSecret      string
	Blacklist      *cache.Blacklist
	CookieSameSite SameSiteMode
	Sessions       middleware.SessionChecker
	UserService    users.Service
	ResetStore     *ResetStore
	Mailer         Mailer
	BaseURL        string
	TrustedProxies []string
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

	rg.POST("/auth/logout", middleware.AuthRequired(opts.JWTSecret, opts.Blacklist, opts.Sessions), h.Logout)
	rg.GET("/me", middleware.AuthRequired(opts.JWTSecret, opts.Blacklist, opts.Sessions), h.Me)

	// Password reset routes are registered only when fully wired (store +
	// mailer + public base URL); test routers that omit them keep the legacy
	// surface instead of half-working endpoints.
	if opts.ResetStore != nil && opts.Mailer != nil && opts.BaseURL != "" {
		resetIP := middleware.New(20, time.Minute)
		resetH := NewResetHandler(
			opts.UserService, opts.ResetStore, opts.Mailer, opts.BaseURL,
			middleware.New(5, time.Minute),
			opts.CookieSameSite, opts.TrustedProxies,
		)
		rg.POST("/auth/password/request", resetIP.Middleware(), resetH.Request)
		rg.POST("/auth/password/reset", resetIP.Middleware(), resetH.Reset)
	}
}

// globalKey is a constant-function key so the MiddlewareKeyed limiter applies a
// single counter across all clients rather than per source IP.
func globalKey(name string) func(*gin.Context) string {
	return func(*gin.Context) string { return name }
}
