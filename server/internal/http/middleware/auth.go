package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
	"github.com/jojianya/sweetspot247-backend/internal/platform/cache"
	"github.com/jojianya/sweetspot247-backend/pkg/jwt"
)

const (
	CtxUserID    = "user_id"
	CtxJWTClaims = "jwt_claims"
	// CtxRole carries the caller's live role, resolved once by the session
	// check, so authorization helpers reuse it instead of querying again.
	CtxRole = "user_role"
)

// SessionState is the per-user session posture: the instant before which
// tokens are dead, plus the live role for context caching.
type SessionState struct {
	ValidAfter time.Time
	Role       string
}

// SessionChecker loads the session posture for a user. Implemented by the
// user service (one indexed PK lookup); the middleware passes it through
// RouteOptions, and tests stub it or omit it.
type SessionChecker interface {
	CheckSession(ctx context.Context, userID string) (SessionState, error)
}

func oneChecker(checker []SessionChecker) SessionChecker {
	if len(checker) == 0 || checker[0] == nil {
		return nil
	}
	return checker[0]
}

// parseBearerClaims validates the session from either the Authorization header
// (Bearer token) or the httpOnly session cookie. The cookie is the primary
// mechanism; the Bearer header is still accepted for API clients that cannot
// use cookies. When the request is invalid it returns ok=false with the exact
// client-facing reason; callers decide whether to abort or continue.
func parseBearerClaims(c *gin.Context, jwtSecret string, bl *cache.Blacklist, checker SessionChecker) (claims *jwt.Claims, reason string, ok bool) {
	// Try the cookie first — it is the primary session mechanism.
	if cookie, err := c.Cookie("session_token"); err == nil && cookie != "" {
		claims, reason, ok = validateToken(c, jwtSecret, bl, checker, cookie)
		if ok {
			return claims, "", true
		}
		// Cookie exists but is invalid — fall through to try Bearer.
	}

	// Fall back to the Bearer header for API clients.
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		if reason == "" {
			reason = "missing or invalid Authorization header"
		}
		return nil, reason, false
	}

	tokenString := strings.TrimPrefix(header, "Bearer ")
	return validateToken(c, jwtSecret, bl, checker, tokenString)
}

// validateToken checks a JWT against the secret, the blacklist, and the
// per-user session floor (sessions_valid_after). A nil checker preserves the
// legacy blacklist-only behavior for tests that do not wire a user service.
func validateToken(c *gin.Context, jwtSecret string, bl *cache.Blacklist, checker SessionChecker, tokenString string) (claims *jwt.Claims, reason string, ok bool) {
	claims, err := jwt.Validate(jwtSecret, tokenString)
	if err != nil {
		return nil, "invalid or expired token", false
	}

	if bl != nil {
		revoked, err := bl.IsRevoked(c.Request.Context(), claims.ID)
		if err != nil {
			// Fail closed. If the blacklist cannot be consulted we cannot know
			// whether this token was revoked, and accepting it would silently
			// un-revoke every logged-out session for the life of the token.
			// Denying is the safe default; a Redis outage reads as "sign in
			// again" rather than as a silently resurrected session.
			slog.Default().Error("blacklist check failed, denying request", "error", err.Error(), "jti", claims.ID)
			return nil, "session verification unavailable, please try again", false
		}
		if revoked {
			return nil, "session was logged out, please sign in again", false
		}
	}

	if checker != nil {
		state, err := checker.CheckSession(c.Request.Context(), claims.UserID)
		if err != nil {
			// Fail closed like the blacklist: without the revocation floor we
			// cannot know whether a password reset killed this session.
			slog.Default().Error("session check failed, denying request", "error", err.Error(), "user_id", claims.UserID)
			return nil, "session verification unavailable, please try again", false
		}
		// The floor keeps full precision in storage, but JWT issued-at is
		// whole seconds: compare at second precision so a login minted in
		// the reset's own second is not read as "before". This leaves up to
		// a one-second window where a pre-reset token from the same second
		// still passes; see docs/SECURITY.md.
		if claims.IssuedAt != nil && claims.IssuedAt.Time.Before(state.ValidAfter.Truncate(time.Second)) {
			return nil, "session was reset, please sign in again", false
		}
		applyRole(c, state.Role)
	}

	return claims, "", true
}

// applyClaims stores the validated identity on the request context.
//
// The role is deliberately not stored here. A role baked into the token at
// login goes stale, so authorization reads it live from the database via
// users.CurrentRole / users.IsModerator instead.
func applyClaims(c *gin.Context, claims *jwt.Claims) {
	c.Set(CtxUserID, claims.UserID)
	c.Set(CtxJWTClaims, claims)
}

// applyRole caches the live role for authorization helpers. Empty roles are
// never cached: CurrentRole treats a missing value as "look it up".
func applyRole(c *gin.Context, role string) {
	if role != "" {
		c.Set(CtxRole, role)
	}
}

func AuthRequired(jwtSecret string, bl *cache.Blacklist, checker ...SessionChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, reason, ok := parseBearerClaims(c, jwtSecret, bl, oneChecker(checker))
		if !ok {
			response.AbortError(c, http.StatusUnauthorized, reason)
			return
		}
		applyClaims(c, claims)
		c.Next()
	}
}

func OptionalAuth(jwtSecret string, bl *cache.Blacklist, checker ...SessionChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, _, ok := parseBearerClaims(c, jwtSecret, bl, oneChecker(checker))
		if !ok {
			c.Next()
			return
		}
		applyClaims(c, claims)
		c.Next()
	}
}

func GetUserID(c *gin.Context) string {
	return c.GetString(CtxUserID)
}
