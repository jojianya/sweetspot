package auth

import (
	"net/http"
	"time"
)

// CookieName is the name of the httpOnly session cookie.
const CookieName = "session_token"

// SameSiteMode represents the SameSite attribute for cookies.
type SameSiteMode string

const (
	SameSiteStrict SameSiteMode = "strict"
	SameSiteLax    SameSiteMode = "lax"
)

// toHTTPSameSite converts a SameSiteMode to http.SameSite value.
func toHTTPSameSite(mode SameSiteMode) http.SameSite {
	switch mode {
	case SameSiteLax:
		return http.SameSiteLaxMode
	default:
		return http.SameSiteStrictMode
	}
}

// SetSessionCookie sets the session token as an httpOnly, Secure, SameSite=Strict
// cookie. httpOnly prevents JavaScript access (XSS protection); SameSite=Strict
// prevents the cookie from being sent on cross-site requests (CSRF protection).
//
// The Secure flag is set based on the request scheme: Secure cookies are only
// sent over HTTPS, so enabling them on plain HTTP would cause the browser to
// silently drop the cookie — breaking authentication entirely.
func SetSessionCookie(w http.ResponseWriter, r *http.Request, token string, maxAge time.Duration, sameSite SameSiteMode) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(maxAge.Seconds()),
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: toHTTPSameSite(sameSite),
	})
}

// ClearSessionCookie removes the session cookie by setting MaxAge to -1.
func ClearSessionCookie(w http.ResponseWriter, r *http.Request, sameSite SameSiteMode) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: toHTTPSameSite(sameSite),
	})
}

// isSecureRequest reports whether the request arrived over HTTPS.
func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	// Check for a reverse proxy that terminated TLS.
	if r.Header.Get("X-Forwarded-Proto") == "https" {
		return true
	}
	return false
}
