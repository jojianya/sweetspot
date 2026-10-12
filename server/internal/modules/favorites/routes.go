package favorites

import (
	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/http/validid"
	"github.com/jojianya/sweetspot247-backend/internal/platform/cache"
)

type RouteOptions struct {
	JWTSecret string
	Blacklist *cache.Blacklist
	Sessions  middleware.SessionChecker
	// Limits overrides the save/unsave budgets. The zero value keeps the
	// production defaults; a test sets PerUser and PerIP to 1 so one extra
	// request proves each limiter is attached.
	Limits middleware.Limits
}

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opts RouteOptions) {
	limits := opts.Limits.WithDefaults()
	perUser := middleware.New(limits.PerUser, limits.Window)
	perIP := middleware.New(limits.PerIP, limits.Window)

	// userCapped keys on the account, skipping anonymous callers (there are
	// none: AuthRequired runs first, so this is defensive).
	userCapped := perUser.MiddlewareUserKeyed("favorite:user:")

	// GET routes are deliberately unthrottled: they are reads, bounded by the
	// caller's own account, and adding a limiter here would throttle scrolling
	// a saved-pins list.

	rg.GET("/favorites", middleware.AuthRequired(opts.JWTSecret, opts.Blacklist, opts.Sessions), h.GetSaved)
	rg.GET("/favorites/ids", middleware.AuthRequired(opts.JWTSecret, opts.Blacklist, opts.Sessions), h.GetSavedIDs)
	// One pin, one EXISTS probe: the client asks this instead of GET
	// /favorites/ids (which returns every saved id) to learn a single answer.
	rg.GET("/favorites/:id/saved", middleware.AuthRequired(opts.JWTSecret, opts.Blacklist, opts.Sessions), h.IsSaved)

	// Mutating routes: validid first so a malformed id answers 400 without
	// spending budget, then auth, then the per-IP backstop, then the per-user
	// limit — the same ordering the Good-spot routes use.
	rg.PUT("/favorites/:id", validid.Middleware(), middleware.AuthRequired(opts.JWTSecret, opts.Blacklist, opts.Sessions), perIP.Middleware(), userCapped, h.Save)
	rg.DELETE("/favorites/:id", validid.Middleware(), middleware.AuthRequired(opts.JWTSecret, opts.Blacklist, opts.Sessions), perIP.Middleware(), userCapped, h.Unsave)
}
