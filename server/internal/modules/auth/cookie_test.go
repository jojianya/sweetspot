package auth

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

func requestWithPeer(path, remoteAddr string, xfp string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = remoteAddr
	if xfp != "" {
		req.Header.Set("X-Forwarded-Proto", xfp)
	}
	return req
}

func TestIsSecureRequest(t *testing.T) {
	trusted := parseTrustedProxies([]string{"10.0.0.1", "172.18.0.0/16"})

	if !isSecureRequest(&http.Request{TLS: &tls.ConnectionState{}}, nil) {
		t.Error("direct TLS must be secure without any trusted proxies")
	}
	if isSecureRequest(requestWithPeer("/", "192.0.2.1:1234", ""), nil) {
		t.Error("plain HTTP without X-Forwarded-Proto must not be secure")
	}
	// Spoofed header from an untrusted peer must be ignored: otherwise any
	// client could flip the Secure flag and silently break (or fix) sessions.
	if isSecureRequest(requestWithPeer("/", "192.0.2.1:1234", "https"), nil) {
		t.Error("X-Forwarded-Proto from untrusted peer must be ignored")
	}
	if isSecureRequest(requestWithPeer("/", "192.0.2.1:1234", "https"), trusted) {
		t.Error("X-Forwarded-Proto from non-proxy peer must be ignored even with proxies configured")
	}
	if !isSecureRequest(requestWithPeer("/", "10.0.0.1:4321", "https"), trusted) {
		t.Error("X-Forwarded-Proto=https from trusted proxy IP must be secure")
	}
	if !isSecureRequest(requestWithPeer("/", "172.18.5.4:4321", "https"), trusted) {
		t.Error("X-Forwarded-Proto=https from trusted proxy CIDR must be secure")
	}
	if isSecureRequest(requestWithPeer("/", "10.0.0.1:4321", "http"), trusted) {
		t.Error("X-Forwarded-Proto=http from trusted proxy must not be secure")
	}
}

func TestSetSessionCookieSecureFlag(t *testing.T) {
	trusted := parseTrustedProxies([]string{"10.0.0.1"})

	w := httptest.NewRecorder()
	SetSessionCookie(w, requestWithPeer("/", "10.0.0.1:1", "https"), "tok", 60, SameSiteStrict, trusted)
	if got := w.Result().Cookies()[0].Secure; !got {
		t.Error("expected Secure cookie behind trusted TLS-terminating proxy")
	}

	w = httptest.NewRecorder()
	SetSessionCookie(w, requestWithPeer("/", "192.0.2.9:1", "https"), "tok", 60, SameSiteStrict, trusted)
	if got := w.Result().Cookies()[0].Secure; got {
		t.Error("spoofed X-Forwarded-Proto must not set Secure")
	}
}
