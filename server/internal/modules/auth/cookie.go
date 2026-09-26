package auth

import (
	"net/http"
	"time"
)

// CookieName is the name of the httpOnly session cookie.
const CookieName = "session_token"

// SetSessionCookie sets the session token as an httpOnly, Secure, SameSite=Strict
// cookie. httpOnly prevents JavaScript access (XSS protection); SameSite=Strict
// prevents the cookie from being sent on cross-site requests (CSRF protection).
func SetSessionCookie(w http.ResponseWriter, token string, maxAge time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(maxAge.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearSessionCookie removes the session cookie by setting MaxAge to -1.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}
