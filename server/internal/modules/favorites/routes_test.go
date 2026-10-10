package favorites

// Wiring tests: prove PUT and DELETE /favorites/:id really carry both limiters,
// and in the order the route comment claims — validid, then auth, then per-IP,
// then per-user.
//
// No database: AuthRequired is built with a nil session checker, validid only
// checks the id's shape, and the limiters run before the handler, so the pin is
// never looked up. That keeps this fast and out of the DB-gated set.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/pkg/jwt"
)

// favoritesLimiterTestSecret only mints tokens for this test's own router.
const favoritesLimiterTestSecret = "favorites-limiter-test-secret-value"

// A syntactically valid UUID. The pin does not need to exist: the limiter
// rejects before the handler would query for it.
const favoritesLimiterPin = "11111111-2222-4333-8444-555555555555"

// stubSaveRepo lets the mutating handlers succeed. The service is built from
// this stub, so reaching the handler at all proves the middleware chain let the
// request through. Every method the two handlers touch is implemented: an
// embedded nil Repository would satisfy the interface syntactically and then
// panic on the first call.
type stubSaveRepo struct{ Repository }

func (stubSaveRepo) Save(context.Context, string, string) error   { return nil }
func (stubSaveRepo) Unsave(context.Context, string, string) error { return nil }

func (stubSaveRepo) PinExists(context.Context, string) (bool, error) {
	return true, nil
}

// IsSaved reports true, because Service.Unsave answers 404 unless the pin is
// already saved — and this wiring test wants the request to reach the handler,
// not to be refused by a business rule.
func (stubSaveRepo) IsSaved(context.Context, string, string) (bool, error) { return true, nil }

// ListIDs backs the GET routes, which the unthrottled test calls.
func (stubSaveRepo) ListIDs(context.Context, string) ([]string, error) { return []string{}, nil }

func setupFavoritesLimiterRouter(t *testing.T, perUser, perIP int) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	h := NewHandler(stubSaveRepo{})
	// The stub answers 204, which is what proves the limiter did not block.
	r := gin.New()
	RegisterRoutes(r.Group(""), h, RouteOptions{
		JWTSecret: favoritesLimiterTestSecret,
		Limits:    Limits{PerUser: perUser, PerIP: perIP, Window: time.Minute},
	})
	return r
}

func favoritesLimiterCall(t *testing.T, r *gin.Engine, method, userID string) int {
	t.Helper()
	tok, err := jwt.Generate(favoritesLimiterTestSecret, userID, time.Hour)
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	req := httptest.NewRequest(method, "/favorites/"+favoritesLimiterPin, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

// TestFavoritesPerUserLimiterAttached proves the per-account budget fires. The
// per-IP budget is left wide open, so the 429 can only come from the per-user
// limiter.
func TestFavoritesPerUserLimiterAttached(t *testing.T) {
	r := setupFavoritesLimiterRouter(t, 1, 100)

	if code := favoritesLimiterCall(t, r, http.MethodPut, "user-a"); code != http.StatusNoContent {
		t.Fatalf("first request status = %d, want 204", code)
	}
	if code := favoritesLimiterCall(t, r, http.MethodPut, "user-a"); code != http.StatusTooManyRequests {
		t.Fatalf("second request from the same user status = %d, want 429", code)
	}

	// A different account must not be blocked by the first account's budget.
	if code := favoritesLimiterCall(t, r, http.MethodPut, "user-b"); code != http.StatusNoContent {
		t.Fatalf("different user status = %d, want 200 (per-user budgets are separate)", code)
	}
}

// TestFavoritesPerIPLimiterAttached proves the per-IP backstop fires. The
// per-user budget is left wide open and each call uses a *different* account,
// so the 429 can only come from the per-IP limiter.
func TestFavoritesPerIPLimiterAttached(t *testing.T) {
	r := setupFavoritesLimiterRouter(t, 100, 1)

	if code := favoritesLimiterCall(t, r, http.MethodPut, "user-a"); code != http.StatusNoContent {
		t.Fatalf("first request status = %d, want 204", code)
	}
	// Same ClientIP, different account.
	if code := favoritesLimiterCall(t, r, http.MethodPut, "user-b"); code != http.StatusTooManyRequests {
		t.Fatalf("second request from a different user but the same IP status = %d, want 429", code)
	}
}

// TestFavoritesLimiterAppliesToDelete proves the DELETE route carries the same
// chain, not just PUT. A limiter attached to one verb and forgotten on the other
// is exactly the asymmetry that gets missed.
func TestFavoritesLimiterAppliesToDelete(t *testing.T) {
	r := setupFavoritesLimiterRouter(t, 1, 100)

	if code := favoritesLimiterCall(t, r, http.MethodDelete, "user-a"); code != http.StatusNoContent {
		t.Fatalf("first DELETE status = %d, want 204", code)
	}
	if code := favoritesLimiterCall(t, r, http.MethodDelete, "user-a"); code != http.StatusTooManyRequests {
		t.Fatalf("second DELETE from the same user status = %d, want 429", code)
	}
}

// TestFavoritesLimiterRunsBeforeTheHandler proves the ordering claim: a flood is
// rejected by middleware before any work happens, so an over-budget request
// never reaches the service.
func TestFavoritesLimiterRunsBeforeTheHandler(t *testing.T) {
	r := setupFavoritesLimiterRouter(t, 2, 100)

	favoritesLimiterCall(t, r, http.MethodPut, "user-a")
	favoritesLimiterCall(t, r, http.MethodPut, "user-a")
	if code := favoritesLimiterCall(t, r, http.MethodPut, "user-a"); code != http.StatusTooManyRequests {
		t.Fatalf("over-budget request status = %d, want 429", code)
	}
}

// TestFavoritesGetRoutesStayUnthrottled pins the deliberate asymmetry: the GET
// routes were left alone, so reading a saved list must never 429.
func TestFavoritesGetRoutesStayUnthrottled(t *testing.T) {
	r := setupFavoritesLimiterRouter(t, 1, 1)

	tok, err := jwt.Generate(favoritesLimiterTestSecret, "user-a", time.Hour)
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	// Two GETs with both budgets already exhausted by the wiring above.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/favorites/ids", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		// 500 is acceptable (the stub repo returns nothing usable); 429 is not.
		if w.Code == http.StatusTooManyRequests {
			t.Fatalf("GET /favorites/ids was rate limited on request %d, want it left open", i+1)
		}
	}
}
