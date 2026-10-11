package endpointtest

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/modules/auth"
	users "github.com/jojianya/sweetspot247-backend/internal/modules/user"
)

// TestTrustedProxyClientIP verifies that when TRUSTED_PROXIES covers the
// connecting peer, the X-Forwarded-For header determines ClientIP.
// Gin returns the rightmost untrusted IP in the X-Forwarded-For chain.
func TestTrustedProxyClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Trust the test proxy IP
	_ = r.SetTrustedProxies([]string{"10.0.0.1"})

	r.GET("/ip", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"client_ip": c.ClientIP()})
	})

	// Request from trusted proxy with X-Forwarded-For chain
	// Chain: client(203.0.113.7) -> proxy1(198.51.100.2) -> trusted(10.0.0.1)
	// Gin returns the rightmost untrusted IP: 198.51.100.2
	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 198.51.100.2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var body struct {
		ClientIP string `json:"client_ip"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Gin returns the rightmost untrusted IP in the chain
	if body.ClientIP != "198.51.100.2" {
		t.Errorf("ClientIP = %q, want 198.51.100.2 (rightmost untrusted in chain)", body.ClientIP)
	}
}

// TestUntrustedProxyClientIP verifies that when the peer is NOT in
// TRUSTED_PROXIES and no X-Forwarded-For is present, ClientIP is the peer.
func TestUntrustedProxyClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Trust only 10.0.0.1
	_ = r.SetTrustedProxies([]string{"10.0.0.1"})

	r.GET("/ip", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"client_ip": c.ClientIP()})
	})

	// Request from UNTRUSTED proxy (192.0.2.50) WITHOUT X-Forwarded-For
	// (an untrusted proxy wouldn't be adding headers we trust)
	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = "192.0.2.50:12345"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var body struct {
		ClientIP string `json:"client_ip"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Should be the actual peer since no X-Forwarded-For is present
	if body.ClientIP != "192.0.2.50" {
		t.Errorf("ClientIP = %q, want 192.0.2.50 (peer IP)", body.ClientIP)
	}
}

// TestTrustedProxyLockoutSeparation verifies that different forwarded client
// IPs through the SAME trusted proxy get SEPARATE login lockout counters.
func TestTrustedProxyLockoutSeparation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Trust the proxy
	_ = r.SetTrustedProxies([]string{"10.0.0.1"})

	// Setup auth with a 3-attempt limit
	lim := middleware.New(3, time.Minute)
	hash, _ := passwordHash("password123")
	usersSvc := &mockUserService{
		byEmail: map[string]users.User{
			"alice@example.com": {ID: "u1", Email: "alice@example.com", Username: "alice", PasswordHash: hash, Role: users.RoleUser},
			"bob@example.com":   {ID: "u2", Email: "bob@example.com", Username: "bob", PasswordHash: hash, Role: users.RoleUser},
		},
	}
	authH := auth.NewHandler(auth.NewService(usersSvc, testSecret), nil, lim, nil, auth.SameSiteStrict, []string{"10.0.0.1"})
	r.POST("/auth/login", authH.Login)

	// Attacker uses X-Forwarded-For: 203.0.113.10
	for i := 0; i < 3; i++ {
		w := doLoginWithForwarded(r, `{"identifier":"alice@example.com","password":"wrong"}`, "10.0.0.1", "203.0.113.10")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attacker attempt %d: expected 401, got %d", i+1, w.Code)
		}
	}
	// 4th attempt from same forwarded IP should lock
	w := doLoginWithForwarded(r, `{"identifier":"alice@example.com","password":"wrong"}`, "10.0.0.1", "203.0.113.10")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected lockout for attacker, got %d", w.Code)
	}

	// Different forwarded IP (203.0.113.20) should NOT be locked
	w = doLoginWithForwarded(r, `{"identifier":"bob@example.com","password":"wrong"}`, "10.0.0.1", "203.0.113.20")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("victim from different forwarded IP should not be locked, got %d", w.Code)
	}
}

// TestTrustedProxySecureCookie verifies the session cookie Secure flag:
// - Trusted peer + X-Forwarded-Proto: https → Secure=true
// - Untrusted peer + X-Forwarded-Proto: https → Secure=false
// - Direct TLS → Secure=true
// - Plain HTTP → Secure=false
func TestTrustedProxySecureCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		remoteAddr  string
		protoHeader string
		tls         bool
		wantSecure  bool
	}{
		{
			name:        "trusted peer with https proto",
			remoteAddr:  "10.0.0.1:12345",
			protoHeader: "https",
			tls:         false,
			wantSecure:  true,
		},
		{
			name:        "untrusted peer with https proto (spoofed)",
			remoteAddr:  "192.0.2.50:12345",
			protoHeader: "https",
			tls:         false,
			wantSecure:  false,
		},
		{
			name:        "direct TLS connection",
			remoteAddr:  "203.0.113.7:12345",
			protoHeader: "",
			tls:         true,
			wantSecure:  true,
		},
		{
			name:        "plain HTTP from trusted peer",
			remoteAddr:  "10.0.0.1:12345",
			protoHeader: "http",
			tls:         false,
			wantSecure:  false,
		},
		{
			name:        "plain HTTP from untrusted peer",
			remoteAddr:  "192.0.2.50:12345",
			protoHeader: "http",
			tls:         false,
			wantSecure:  false,
		},
		{
			name:        "trusted peer with no proto header (plain HTTP)",
			remoteAddr:  "10.0.0.1:12345",
			protoHeader: "",
			tls:         false,
			wantSecure:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			r := gin.New()
			_ = r.SetTrustedProxies([]string{"10.0.0.1"})

			// Create a handler that just returns the cookie
			hash, _ := passwordHash("password123")
			usersSvc := &mockUserService{
				byEmail: map[string]users.User{
					"test@example.com": {ID: "u1", Email: "test@example.com", Username: "test", PasswordHash: hash, Role: users.RoleUser},
				},
			}
			authH := auth.NewHandler(auth.NewService(usersSvc, testSecret), nil, nil, nil, auth.SameSiteStrict, []string{"10.0.0.1"})
			r.POST("/auth/login", authH.Login)

			var req *http.Request
			if tc.tls {
				req = httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"identifier":"test@example.com","password":"password123"}`))
				req.TLS = &tls.ConnectionState{}
			} else {
				req = httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"identifier":"test@example.com","password":"password123"}`))
			}
			req.RemoteAddr = tc.remoteAddr
			if tc.protoHeader != "" {
				req.Header.Set("X-Forwarded-Proto", tc.protoHeader)
			}
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("%s: status = %d, want 200", tc.name, w.Code)
			}

			cookies := w.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("expected exactly one cookie, got %d", len(cookies))
			}
			c := cookies[0]
			if c.Secure != tc.wantSecure {
				t.Errorf("%s: Secure = %v, want %v", tc.name, c.Secure, tc.wantSecure)
			}
		})
	}
}

// doLoginWithForwarded posts a login with a custom X-Forwarded-For header.
func doLoginWithForwarded(r *gin.Engine, body, remoteAddr, forwardedFor string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr
	req.Header.Set("X-Forwarded-For", forwardedFor)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestPerIPBucketsSeparateAcrossForwardedClients is the C-01 regression:
// two browsers behind the same trusted proxy (nginx) must not share one
// rate-limit bucket, and a spoofed X-Forwarded-For from an untrusted peer
// must not mint a fresh bucket per header value.
func TestPerIPBucketsSeparateAcrossForwardedClients(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	_ = r.SetTrustedProxies([]string{"10.89.0.0/24"})

	lim := middleware.New(1, time.Minute)
	r.POST("/limited", lim.Middleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ip": c.ClientIP()})
	})

	post := func(remoteAddr, xff string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/limited", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = remoteAddr
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// Two real clients through the same nginx peer: separate buckets.
	if w := post("10.89.0.2:1111", "203.0.113.9"); w.Code != http.StatusOK {
		t.Fatalf("first client: expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	if w := post("10.89.0.2:2222", "198.51.100.7"); w.Code != http.StatusOK {
		t.Fatalf("second client behind same proxy: expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	if w := post("10.89.0.2:3333", "203.0.113.9"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("repeat client: expected 429, got %d (%s)", w.Code, w.Body.String())
	}

	// Untrusted peer: the spoofed header is ignored, so both hits land in
	// the peer bucket and the second is limited.
	if w := post("192.0.2.50:1111", ""); w.Code != http.StatusOK {
		t.Fatalf("untrusted first hit: expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	if w := post("192.0.2.50:2222", "203.0.113.99"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("untrusted spoofed hit: expected 429, got %d (%s)", w.Code, w.Body.String())
	}
}
