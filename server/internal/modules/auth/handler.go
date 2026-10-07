package auth

import (
	"errors"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/internal/platform/cache"
	"github.com/jojianya/sweetspot247-backend/pkg/jwt"
)

type Handler struct {
	service  Service
	bl       *cache.Blacklist
	emailLim *middleware.Limiter
	sameSite SameSiteMode
	trusted  []*net.IPNet
}

func NewHandler(service Service, bl *cache.Blacklist, emailLim *middleware.Limiter, sameSite SameSiteMode, trustedProxies []string) *Handler {
	return &Handler{service: service, bl: bl, emailLim: emailLim, sameSite: sameSite, trusted: parseTrustedProxies(trustedProxies)}
}

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// The binding tags cover presence and length; the character rule lives in
	// the users module so registration and profile rename cannot drift apart.
	// Checked here, next to the bind error, so a bad username is a 400 like any
	// other input problem instead of falling through to the 500 branch.
	if err := users.ValidateUsername(strings.TrimSpace(req.Username)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	u, token, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrConflict) {
			response.Conflict(c, "email or username already taken")
			return
		}
		if errors.Is(err, ErrInvalidPassword) {
			// Client input, not a server fault. Return the specific reason so the
			// user can tell a short password from an over-long one.
			response.BadRequest(c, err.Error())
			return
		}
		response.Internal(c, "auth: register", err)
		return
	}

	SetSessionCookie(c.Writer, c.Request, token, tokenExpiry, h.sameSite, h.trusted)
	response.Created(c, gin.H{"user": u})
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Key the lockout on the client IP *and* the identifier. Keyed on the
	// identifier alone, anyone who can reach this endpoint could spend the real
	// owner's budget with five wrong passwords and lock that person out of their
	// own account from anywhere — a denial of service aimed at one user rather
	// than at the endpoint. Pairing the two means failures only ever lock out
	// the (IP, identifier) pair that produced them; the per-IP and global
	// middleware limits still bound one attacker from grinding through many
	// identifiers.
	lockKey := loginLockKey(c.ClientIP(), req.Identifier)
	if h.emailLim != nil && h.emailLim.Locked(lockKey) {
		response.TooManyRequests(c, "account locked, try again later")
		return
	}

	u, token, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			if h.emailLim != nil {
				h.emailLim.AllowKey(lockKey)
			}
			response.Unauthorized(c, "invalid email, username, or password")
			return
		}
		response.Internal(c, "auth: login", err)
		return
	}

	if h.emailLim != nil {
		// Success clears only this IP+identifier counter, so one user signing
		// in does not release a lockout an attacker earned from their own IP.
		h.emailLim.Reset(lockKey)
	}

	SetSessionCookie(c.Writer, c.Request, token, tokenExpiry, h.sameSite, h.trusted)
	response.OK(c, gin.H{"user": u})
}

func (h *Handler) Logout(c *gin.Context) {
	claims, ok := c.Get(middleware.CtxJWTClaims)
	if !ok {
		response.Unauthorized(c, "invalid or expired token")
		return
	}

	// Clear the cookie first so the client is logged out regardless of
	// whether the revoke succeeds. The token will expire naturally if the
	// revoke fails, but the client should not be told the logout failed.
	ClearSessionCookie(c.Writer, c.Request, h.sameSite, h.trusted)

	jwtClaims := claims.(*jwt.Claims)
	ttl := time.Until(jwtClaims.ExpiresAt.Time)
	if err := h.bl.Revoke(c.Request.Context(), jwtClaims.ID, ttl); err != nil {
		// Log the failure but do not fail the request. The cookie is already
		// cleared, and the token will expire on its own. A Redis outage should
		// not prevent logout.
		slog.Warn("logout: failed to revoke token", "error", err.Error())
	}

	response.OK(c, gin.H{"message": "logged out"})
}

func (h *Handler) Me(c *gin.Context) {
	role, err := h.service.Role(c.Request.Context(), middleware.GetUserID(c))
	if err != nil {
		response.Internal(c, "auth: me", err, "user_id", middleware.GetUserID(c))
		return
	}
	response.OK(c, gin.H{
		"user_id": middleware.GetUserID(c),
		"role":    role,
	})
}

// loginLockKey is the failure-counter key for one login attempt: the client IP
// plus the identifier being tried.
//
// The identifier is lowercased because the account lookup treats an email
// case-insensitively (GetByLogin compares against LOWER(email)), so "USER@x.com"
// and "user@x.com" are the same account. Without folding case here an attacker
// could mint a fresh counter per variant and never reach the threshold. This
// also merges case-distinct usernames into one counter, which can only make the
// lockout slightly eager and only for the attacker's own IP.
func loginLockKey(ip, identifier string) string {
	return ip + "|" + strings.ToLower(identifier)
}
