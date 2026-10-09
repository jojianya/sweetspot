package auth

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	users "github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/pkg/password"
)

// productionSameSite is the mode a production deployment runs with:
// COOKIE_SAMESITE unset, so config falls back to "strict"
// (see config.Load and docker-compose.prod.yml, both pinned by
// TestCookieSameSiteDefaultsToStrict).
const productionSameSite = SameSiteStrict

// TestSessionCookieIsStrictAndHttpOnlyUnderProductionConfig asserts the
// attributes the app's CSRF defence actually rests on. There is no anti-CSRF
// token anywhere in this service, so the cookie being SameSite=Strict and
// httpOnly is not a nicety — it is the control. If a change made the session
// cookie Lax in production, cross-site top-level GETs would start carrying it
// and this test is what would notice.
//
// Secure is asserted separately because it is request-dependent: it is set iff
// the request arrived over TLS (directly, or via a trusted proxy's
// X-Forwarded-Proto), not by configuration.
func TestSessionCookieIsStrictAndHttpOnlyUnderProductionConfig(t *testing.T) {
	hash, err := password.Hash("password123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	svc := &stubUserService{
		byEmail:    map[string]users.User{},
		byUsername: map[string]users.User{},
	}
	svc.byEmail["a@example.com"] = users.User{
		ID: "u-1", Email: "a@example.com", Username: "alice",
		Role: users.RoleUser, PasswordHash: hash,
	}

	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(svc, "test-secret"), nil, nil, nil, productionSameSite, nil)
	r := gin.New()
	r.POST("/auth/login", h.Login)

	// Over TLS, so the production-shaped cookie is the one under test.
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(
		`{"identifier":"a@example.com","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	req.TLS = &tls.ConnectionState{}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200 (%s)", w.Code, w.Body.String())
	}

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected exactly one Set-Cookie, got %d", len(cookies))
	}
	c := cookies[0]

	if c.Name != CookieName {
		t.Errorf("cookie name = %q, want %q", c.Name, CookieName)
	}
	if !c.HttpOnly {
		t.Error("session cookie must be HttpOnly")
	}
	if c.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite = %v, want Strict in production", c.SameSite)
	}
	if !c.Secure {
		t.Error("session cookie must be Secure over TLS")
	}
	if c.Path != "/" {
		t.Errorf("Path = %q, want /", c.Path)
	}

	// Assert on the raw header too: SameSite=Strict must actually be written
	// out, not merely implied by a parsed default.
	raw := w.Header().Get("Set-Cookie")
	if !strings.Contains(raw, "SameSite=Strict") {
		t.Errorf("Set-Cookie does not carry SameSite=Strict: %q", raw)
	}
	if !strings.Contains(raw, "HttpOnly") {
		t.Errorf("Set-Cookie does not carry HttpOnly: %q", raw)
	}
}

// The clearing cookie must keep the same attributes, or a logout would leave a
// Lax/Secure-less cookie behind for a browser to keep sending.
func TestClearSessionCookieKeepsStrictAndHttpOnly(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.TLS = &tls.ConnectionState{}
	ClearSessionCookie(w, req, productionSameSite, nil)

	c := w.Result().Cookies()[0]
	if !c.HttpOnly {
		t.Error("cleared cookie must stay HttpOnly")
	}
	if c.SameSite != http.SameSiteStrictMode {
		t.Errorf("cleared cookie SameSite = %v, want Strict", c.SameSite)
	}
	if !strings.Contains(w.Header().Get("Set-Cookie"), "SameSite=Strict") {
		t.Errorf("cleared cookie header: %q", w.Header().Get("Set-Cookie"))
	}
}

// Lax is reachable only by opting in (plain-HTTP LAN development). This test
// documents that the flag does what it claims, so the production default above
// is a deliberate choice rather than the only possible outcome.
func TestLaxModeIsOptInAndReachesTheCookie(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.TLS = &tls.ConnectionState{}
	SetSessionCookie(w, req, "tok", 60, SameSiteLax, nil)

	c := w.Result().Cookies()[0]
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax when explicitly configured", c.SameSite)
	}
	if !c.HttpOnly {
		t.Error("Lax mode must not relax HttpOnly")
	}
}
