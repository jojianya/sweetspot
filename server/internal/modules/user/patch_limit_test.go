package users

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
)

func TestPatchMeRateLimited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := baseUserService()
	h := NewHandler(svc, storage.NewLocal(t.TempDir(), "http://api.test"))
	r := gin.New()
	RegisterRoutes(r.Group(""), h, RouteOptions{JWTSecret: "test-secret-for-rate-limit-1234567890"})

	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPatch, "/users/me", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		r.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d: got 429 too early, want 401 (auth) while budget remains", i+1)
		}
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/users/me", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("11th PATCH /users/me: status = %d, want 429", w.Code)
	}
}
