package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeaders sets hardening headers on every response. It is safe for a
// JSON API because the CSP is intentionally restrictive (no scripts allowed),
// and the app serves images from its own /uploads path only.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; img-src 'self'")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Next()
	}
}
