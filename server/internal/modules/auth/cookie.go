package auth

import (
	"net"
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
// silently drop the cookie — breaking authentication entirely. trusted is the
// parsed TRUSTED_PROXIES list; only those peers may assert HTTPS via
// X-Forwarded-Proto.
func SetSessionCookie(w http.ResponseWriter, r *http.Request, token string, maxAge time.Duration, sameSite SameSiteMode, trusted []*net.IPNet) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(maxAge.Seconds()),
		HttpOnly: true,
		Secure:   isSecureRequest(r, trusted),
		SameSite: toHTTPSameSite(sameSite),
	})
}

// ClearSessionCookie removes the session cookie by setting MaxAge to -1.
func ClearSessionCookie(w http.ResponseWriter, r *http.Request, sameSite SameSiteMode, trusted []*net.IPNet) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isSecureRequest(r, trusted),
		SameSite: toHTTPSameSite(sameSite),
	})
}

// parseTrustedProxies compiles IP/CIDR strings into networks for peer checks.
// Invalid entries are skipped; server config validation already rejects them
// at startup, so this is defense in depth, not the primary gate.
func parseTrustedProxies(proxies []string) []*net.IPNet {
	var out []*net.IPNet
	for _, entry := range proxies {
		if _, cidr, err := net.ParseCIDR(entry); err == nil {
			out = append(out, cidr)
			continue
		}
		if ip := net.ParseIP(entry); ip != nil {
			bits := 128
			if ip.To4() != nil {
				bits = 32
			}
			out = append(out, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
		}
	}
	return out
}

// trustedPeer reports whether the direct TCP peer belongs to the trusted
// proxy networks.
func trustedPeer(r *http.Request, trusted []*net.IPNet) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, cidr := range trusted {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// isSecureRequest reports whether the request arrived over HTTPS: either a
// direct TLS connection, or plain HTTP from a trusted proxy that asserts
// X-Forwarded-Proto: https. A spoofed header from any other peer is ignored,
// so it can neither force Secure on (dropping the cookie) nor, by absence,
// strip it.
func isSecureRequest(r *http.Request, trusted []*net.IPNet) bool {
	if r.TLS != nil {
		return true
	}
	// Check for a reverse proxy that terminated TLS, but only when the peer
	// is a proxy we operate. Trusting this header from anyone would let an
	// arbitrary client flip the Secure flag on the session cookie.
	if r.Header.Get("X-Forwarded-Proto") == "https" && trustedPeer(r, trusted) {
		return true
	}
	return false
}
