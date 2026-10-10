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
}

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opts RouteOptions) {
	authRequired := middleware.AuthRequired(opts.JWTSecret, opts.Blacklist, opts.Sessions)

	// Dual budget, mirroring POST /pins/:id/view: a per-account limit sized for
	// a human tapping through a map, and a per-IP backstop for floods.
	//
	// The per-IP counter alone is not enough. Browsers reaching Go directly
	// through the Next rewrite (dev :3000 without nginx) share one ClientIP, so
	// an IP-only budget would either throttle every user on a NAT or be sized
	// for a crowd and let one account spam. Keying on the account keeps normal
	// use comfortable and still stops one account hammering the toggle.
	//
	// Budgets: 30 toggles a minute per account is far above human pace (a user
	// reacting to a screen of pins at one per second), and 120/min per IP is a
	// backstop, not the primary control. Both limiters are in-process, so they
	// are per-replica and do not hold across multiple server instances — the
	// documented tradeoff for every limiter in this app.
	perUser := middleware.New(30, time.Minute)
	perIP := middleware.New(120, time.Minute)

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
