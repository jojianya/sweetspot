package pins

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
)

func TestPatchPinRateLimited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(baseCreateRepo(), storage.NewLocal(t.TempDir(), "http://api.test"), nil, nil)
	r := gin.New()
	RegisterRoutes(r.Group(""), h, RouteOptions{JWTSecret: "test-secret-for-rate-limit-1234567890"})

	// 10 requests exhaust the per-IP budget; the 11th must be 429.
	// No credentials are sent, so the limiter (placed before auth, like
	// createLimit) is what distinguishes the 11th response.
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPatch, "/pins/"+testOwnerID, nil)
		req.RemoteAddr = "192.0.2.1:1234"
		r.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d: got 429 too early, want 401 (auth) while budget remains", i+1)
		}
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/pins/"+testOwnerID, nil)
	req.RemoteAddr = "192.0.2.1:1234"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("11th PATCH /pins/:id: status = %d, want 429", w.Code)
	}
}
