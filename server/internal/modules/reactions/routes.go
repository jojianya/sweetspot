package reactions

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
	// Limits overrides the toggle budgets. The zero value keeps the production
	// defaults; a test sets PerUser and PerIP to 1 so one extra request is
	// enough to prove each limiter is actually attached.
	Limits Limits
}

// Limits holds the two toggle budgets. Window is shared by both: splitting it
// would let a caller produce a per-user and per-IP window that disagree, which
// is a configuration nobody wants.
type Limits struct {
	PerUser int
	PerIP   int
	Window  time.Duration
}

// Production budgets. Sized for a human tapping through a map, not for a
// script: reacting to a screen of pins one per second is well inside 30/min.
const (
	defaultPerUserReactions = 30
	defaultPerIPReactions   = 120
)

func (l Limits) withDefaults() Limits {
	if l.PerUser <= 0 {
		l.PerUser = defaultPerUserReactions
	}
	if l.PerIP <= 0 {
		l.PerIP = defaultPerIPReactions
	}
	if l.Window <= 0 {
		l.Window = time.Minute
	}
	return l
}

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opts RouteOptions) {
	authRequired := middleware.AuthRequired(opts.JWTSecret, opts.Blacklist, opts.Sessions)
	limits := opts.Limits.withDefaults()

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
	userCapped := func(c *gin.Context) {
		uid := middleware.GetUserID(c)
		if uid == "" {
			c.Next()
			return
		}
		if !perUser.AllowKey("good-spot:user:" + uid) {
			response.AbortError(c, http.StatusTooManyRequests, "too many requests, try again later")
			return
		}
		c.Next()
	}

	// validid first so a malformed id answers 400 without spending budget, then
	// auth so unauthenticated callers are told to log in, then the limiters so a
	// flood is rejected before any work — the same ordering POST /pins uses for
	// its create limiter.
	rg.PUT("/pins/:id/good-spot", validid.Middleware(), authRequired, perIP.Middleware(), userCapped, h.React)
	rg.DELETE("/pins/:id/good-spot", validid.Middleware(), authRequired, perIP.Middleware(), userCapped, h.Unreact)
}
