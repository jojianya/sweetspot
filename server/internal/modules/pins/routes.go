package pins

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/platform/cache"
	"github.com/jojianya/sweetspot247-backend/internal/http/validid"
)

type RouteOptions struct {
	JWTSecret string
	Blacklist *cache.Blacklist
	Sessions  middleware.SessionChecker
}

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opts RouteOptions) {
	rg.GET("/categories", h.ListCategories)
	rg.GET("/pins", h.GetPins)
	rg.GET("/pins/search", h.SearchPins)
	rg.GET("/pins/trending", h.GetTrending)
	rg.GET("/users/:id/pins", validid.Middleware(), h.ListByUser)
	rg.GET("/pins/:id", validid.Middleware(), middleware.OptionalAuth(opts.JWTSecret, opts.Blacklist, opts.Sessions), h.GetPin)

	createLimit := middleware.New(10, time.Minute)
	rg.POST("/pins", createLimit.Middleware(), middleware.AuthRequired(opts.JWTSecret, opts.Blacklist, opts.Sessions), h.CreatePin)

	// View counting reads the session when present but stays open: anonymous
	// visitors get the current count without recording anything. Lenient
	// per-IP cap (a human cannot open 60 pins a minute); floods answer 429.
	viewLimit := middleware.New(60, time.Minute)
	rg.POST("/pins/:id/view", validid.Middleware(), viewLimit.Middleware(), middleware.OptionalAuth(opts.JWTSecret, opts.Blacklist, opts.Sessions), h.RegisterView)

	authRequired := middleware.AuthRequired(opts.JWTSecret, opts.Blacklist, opts.Sessions)
	rg.PATCH("/pins/:id", validid.Middleware(), authRequired, h.UpdatePin)
	rg.DELETE("/pins/:id", validid.Middleware(), authRequired, h.DeletePin)
}
