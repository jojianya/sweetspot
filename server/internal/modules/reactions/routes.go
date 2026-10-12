package reactions

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
	// Limits overrides the toggle budgets. The zero value keeps the production
	// defaults; a test sets PerUser and PerIP to 1 so one extra request is
	// enough to prove each limiter is actually attached.
	Limits middleware.Limits
}

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opts RouteOptions) {
	authRequired := middleware.AuthRequired(opts.JWTSecret, opts.Blacklist, opts.Sessions)
	limits := opts.Limits.WithDefaults()

	// Dual budget, mirroring POST /pins/:id/view: a per-account limit sized for
	// a human tapping through a map, and a per-IP backstop for floods.
	//
	// The per-IP counter alone is not enough. Browsers reaching Go directly
	// through the Next rewrite (dev :3000 without nginx) share one ClientIP, so
	// an IP-only budget would either throttle every user behind one NAT or be
	// sized for a crowd and let a single account spam. Keying on the account
	// keeps normal use comfortable and still stops one account hammering the
	// toggle.
	//
	// Both limiters are in-process, so they are per-replica and do not hold
	// across multiple server instances — the documented tradeoff for every
	// limiter in this app.
	perUser := middleware.New(limits.PerUser, limits.Window)
	perIP := middleware.New(limits.PerIP, limits.Window)

	// userCapped keys on the account, skipping anonymous callers (there are
	// none: AuthRequired runs first, so this is defensive).
	userCapped := perUser.MiddlewareUserKeyed("good-spot:user:")

	// validid first so a malformed id answers 400 without spending budget, then
	// auth so unauthenticated callers are told to log in, then the limiters so a
	// flood is rejected before any work — the same ordering POST /pins uses for
	// its create limiter.
	rg.PUT("/pins/:id/good-spot", validid.Middleware(), authRequired, perIP.Middleware(), userCapped, h.React)
	rg.DELETE("/pins/:id/good-spot", validid.Middleware(), authRequired, perIP.Middleware(), userCapped, h.Unreact)
}
