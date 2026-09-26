package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
	"github.com/jojianya/sweetspot247-backend/internal/platform/cache"
	"github.com/jojianya/sweetspot247-backend/pkg/jwt"
)

const (
	CtxUserID    = "user_id"
	CtxJWTClaims = "jwt_claims"
)

// parseBearerClaims validates the Authorization header against the JWT secret
// and blacklist. When the request is invalid it returns ok=false with the
// exact client-facing reason; callers decide whether to abort or continue.
func parseBearerClaims(c *gin.Context, jwtSecret string, bl *cache.Blacklist) (claims *jwt.Claims, reason string, ok bool) {
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return nil, "missing or invalid Authorization header", false
	}

	tokenString := strings.TrimPrefix(header, "Bearer ")
	claims, err := jwt.Validate(jwtSecret, tokenString)
	if err != nil {
		return nil, "invalid or expired token", false
	}

	if bl != nil {
		revoked, err := bl.IsRevoked(c.Request.Context(), claims.ID)
		if err != nil {
			slog.Default().Warn("blacklist check failed", "error", err.Error())
		} else if revoked {
			return nil, "session was logged out, please sign in again", false
		}
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

func AuthRequired(jwtSecret string, bl *cache.Blacklist) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, reason, ok := parseBearerClaims(c, jwtSecret, bl)
		if !ok {
			response.AbortError(c, http.StatusUnauthorized, reason)
			return
		}
		applyClaims(c, claims)
		c.Next()
	}
}

func OptionalAuth(jwtSecret string, bl *cache.Blacklist) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, _, ok := parseBearerClaims(c, jwtSecret, bl)
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
