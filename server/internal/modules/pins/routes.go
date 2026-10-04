package pins

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
	"github.com/jojianya/sweetspot247-backend/internal/http/validid"
	"github.com/jojianya/sweetspot247-backend/internal/platform/cache"
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
	// visitors get the current count without recording anything. Budgets are
	// per account because browsers share one ClientIP behind the Next proxy
	// (TRUSTED_PROXIES is empty): 60 opens a minute per account is far above
	// human pace, and a 600/min per-IP backstop still stops floods.
	// Anonymous opens never write, so only the backstop applies to them.
	viewBackstop := middleware.New(600, time.Minute)
	viewPerUser := middleware.New(60, time.Minute)
	userCapped := func(c *gin.Context) {
		uid := middleware.GetUserID(c)
		if uid == "" {
			c.Next()
			return
		}
		if !viewPerUser.AllowKey("pin-view:user:" + uid) {
			response.AbortError(c, http.StatusTooManyRequests, "too many requests, try again later")
			return
		}
		c.Next()
	}
	rg.POST("/pins/:id/view", validid.Middleware(), middleware.OptionalAuth(opts.JWTSecret, opts.Blacklist, opts.Sessions), viewBackstop.Middleware(), userCapped, h.RegisterView)

	authRequired := middleware.AuthRequired(opts.JWTSecret, opts.Blacklist, opts.Sessions)
	rg.PATCH("/pins/:id", validid.Middleware(), authRequired, h.UpdatePin)
	rg.DELETE("/pins/:id", validid.Middleware(), authRequired, h.DeletePin)
}
