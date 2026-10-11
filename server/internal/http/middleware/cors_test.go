package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORSAllowsLocalPreviewOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORS())
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://127.0.0.1:3002")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:3002" {
		t.Fatalf("expected local preview origin to be allowed, got %q (status %d)", got, res.Code)
	}
}

// TestCORSSameOriginWithPortBypassesAllowlist pins the contract nginx
// relies on: gin-contrib/cors treats a request as same-origin (no allowlist
// check) when Origin matches Host exactly, port included. nginx must
// therefore forward Host verbatim ($http_host); sending a bare hostname
// turns every browser POST through the proxy into a 403.
func TestCORSSameOriginWithPortBypassesAllowlist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORS("http://localhost:3000", "http://127.0.0.1:3000", "https://goodspot.test:8443"))
	router.POST("/auth/register", func(c *gin.Context) { c.Status(http.StatusOK) })

	post := func(host, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/auth/register", nil)
		req.Host = host
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}

	// Browser via nginx: Host keeps :8443, Origin matches it exactly.
	if res := post("localhost:8443", "https://localhost:8443"); res.Code != http.StatusOK {
		t.Fatalf("same-origin with port: expected 200, got %d", res.Code)
	}
	// Bare-host forwarding (nginx $host): Origin no longer matches Host,
	// so the allowlist applies and the unlisted origin is rejected.
	if res := post("localhost", "https://localhost:8443"); res.Code != http.StatusForbidden {
		t.Fatalf("bare host with unlisted origin: expected 403, got %d", res.Code)
	}
	// The check is not weakened: a foreign origin is rejected even with a
	// verbatim Host.
	if res := post("localhost:8443", "https://evil.example.com"); res.Code != http.StatusForbidden {
		t.Fatalf("foreign origin: expected 403, got %d", res.Code)
	}
}
