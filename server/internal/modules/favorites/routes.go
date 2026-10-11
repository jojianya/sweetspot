package favorites

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
	// Limits overrides the save/unsave budgets. The zero value keeps the
	// production defaults; a test sets PerUser and PerIP to 1 so one extra
	// request proves each limiter is attached. Mirrored from the reactions
	// module so the two toggles cannot drift apart.
	Limits Limits
}

// Limits holds the two save/unsave budgets. Window is shared: splitting it would
// let a caller produce a per-user and per-IP window that disagree, which is a
// configuration nobody wants.
type Limits struct {
	PerUser int
	PerIP   int
	Window  time.Duration
}

// Production budgets, matching the Good-spot routes. Saving is a rarer action
// than reacting (it opens a panel and picks), so the same numbers leave plenty
// of headroom for a human while still stopping a hammering script.
const (
	defaultPerUserFavorites = 30
	defaultPerIPFavorites   = 120
)

func (l Limits) withDefaults() Limits {
	if l.PerUser <= 0 {
		l.PerUser = defaultPerUserFavorites
	}
	if l.PerIP <= 0 {
		l.PerIP = defaultPerIPFavorites
	}
	if l.Window <= 0 {
		l.Window = time.Minute
	}
	return l
}

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opts RouteOptions) {
	limits := opts.Limits.withDefaults()
	perUser := middleware.New(limits.PerUser, limits.Window)
	perIP := middleware.New(limits.PerIP, limits.Window)

	// userCapped keys on the account, skipping anonymous callers (there are
	// none: AuthRequired runs first, so this is defensive).
	userCapped := func(c *gin.Context) {
		uid := middleware.GetUserID(c)
		if uid == "" {
			c.Next()
			return
		}
		if !perUser.AllowKey("favorite:user:" + uid) {
			response.AbortError(c, http.StatusTooManyRequests, "too many requests, try again later")
			return
		}
		c.Next()
	}

	// GET routes are deliberately unthrottled: they are reads, bounded by the
	// caller's own account, and adding a limiter here would throttle scrolling
	// a saved-pins list.

	rg.GET("/favorites", middleware.AuthRequired(opts.JWTSecret, opts.Blacklist, opts.Sessions), h.GetSaved)
	rg.GET("/favorites/ids", middleware.AuthRequired(opts.JWTSecret, opts.Blacklist, opts.Sessions), h.GetSavedIDs)

	// Mutating routes: validid first so a malformed id answers 400 without
	// spending budget, then auth, then the per-IP backstop, then the per-user
	// limit — the same ordering the Good-spot routes use.
	rg.PUT("/favorites/:id", validid.Middleware(), middleware.AuthRequired(opts.JWTSecret, opts.Blacklist, opts.Sessions), perIP.Middleware(), userCapped, h.Save)
	rg.DELETE("/favorites/:id", validid.Middleware(), middleware.AuthRequired(opts.JWTSecret, opts.Blacklist, opts.Sessions), perIP.Middleware(), userCapped, h.Unsave)
}
