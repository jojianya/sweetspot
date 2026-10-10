package reactions

// Wiring tests: prove PUT and DELETE /pins/:id/good-spot really have both
// limiters attached, and that they are attached in the order the route comment
// claims. The handler is a stub that returns 200, so any response other than
// 200 comes from middleware — which is the whole point.
//
// No database: AuthRequired is built with a nil session checker, validid only
// checks the id's shape, and the limiter runs before the handler, so the pin is
// never looked up. That keeps this test fast and out of the DB-gated set.

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/pkg/jwt"
)

// limiterTestSecret only mints tokens for this test's own router.
const limiterTestSecret = "reactions-limiter-test-secret-value"

// A syntactically valid UUID. The pin does not need to exist: the limiter
// rejects before the handler would query for it.
const limiterTestPin = "11111111-2222-4333-8444-555555555555"

func setupLimiterRouter(t *testing.T, perUser, perIP int) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	h := &Handler{
		// A stub whose pin exists and is owned by someone else, so React succeeds
		// and the handler answers 200. Reaching the handler at all proves the
		// middleware chain let the request through; the stub just has to not fail.
		svc: NewService(&stubRepo{exists: true, owner: strPtr("someone-else"), rows: 1, reacted: true}),
	}
	r := gin.New()
	RegisterRoutes(r.Group(""), h, RouteOptions{
		JWTSecret: limiterTestSecret,
		// nil Blacklist and Sessions: no Redis, no session-floor lookup.
		Limits: Limits{PerUser: perUser, PerIP: perIP, Window: time.Minute},
	})
	return r
}

func limiterCall(t *testing.T, r *gin.Engine, method, userID string) int {
	t.Helper()
	tok, err := jwt.Generate(limiterTestSecret, userID, time.Hour)
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	req := httptest.NewRequest(method, "/pins/"+limiterTestPin+"/good-spot", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

// TestGoodSpotPerUserLimiterAttached proves the per-account budget fires. The
// per-IP budget is left wide open, so the 429 can only come from the per-user
// limiter.
func TestGoodSpotPerUserLimiterAttached(t *testing.T) {
	r := setupLimiterRouter(t, 1, 100)

	if code := limiterCall(t, r, http.MethodPut, "user-a"); code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", code)
	}
	if code := limiterCall(t, r, http.MethodPut, "user-a"); code != http.StatusTooManyRequests {
		t.Fatalf("second request from the same user status = %d, want 429", code)
	}

	// A different account must not be blocked by the first account's budget.
	if code := limiterCall(t, r, http.MethodPut, "user-b"); code != http.StatusOK {
		t.Fatalf("different user status = %d, want 200 (per-user budgets are separate)", code)
	}
}

// TestGoodSpotPerIPLimiterAttached proves the per-IP backstop fires. The
// per-user budget is left wide open and each call uses a *different* account,
// so the 429 can only come from the per-IP limiter.
func TestGoodSpotPerIPLimiterAttached(t *testing.T) {
	r := setupLimiterRouter(t, 100, 1)

	if code := limiterCall(t, r, http.MethodPut, "user-a"); code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", code)
	}
	// Same ClientIP (no X-Forwarded-For, and TRUSTED_PROXIES is empty so it
	// would be ignored anyway), different account.
	if code := limiterCall(t, r, http.MethodPut, "user-b"); code != http.StatusTooManyRequests {
		t.Fatalf("second request from a different user but the same IP status = %d, want 429", code)
	}
}

// TestGoodSpotLimiterAppliesToDelete proves the DELETE route carries the same
// chain, not just PUT. A limiter attached to one verb and forgotten on the other
// is exactly the kind of asymmetry that gets missed.
func TestGoodSpotLimiterAppliesToDelete(t *testing.T) {
	r := setupLimiterRouter(t, 1, 100)

	if code := limiterCall(t, r, http.MethodDelete, "user-a"); code != http.StatusOK {
		t.Fatalf("first DELETE status = %d, want 200", code)
	}
	if code := limiterCall(t, r, http.MethodDelete, "user-a"); code != http.StatusTooManyRequests {
		t.Fatalf("second DELETE from the same user status = %d, want 429", code)
	}
}

// TestGoodSpotLimiterRunsBeforeTheHandler proves the ordering claim: a flood is
// rejected by middleware before any work happens, so an over-budget request
// never reaches the service. If the limiter ran after the handler this would
// return 200.
func TestGoodSpotLimiterRunsBeforeTheHandler(t *testing.T) {
	r := setupLimiterRouter(t, 2, 100)

	// Exhaust the budget.
	limiterCall(t, r, http.MethodPut, "user-a")
	limiterCall(t, r, http.MethodPut, "user-a")
	// The third is over budget: the limiter must reject it, not the handler.
	if code := limiterCall(t, r, http.MethodPut, "user-a"); code != http.StatusTooManyRequests {
		t.Fatalf("over-budget request status = %d, want 429", code)
	}
}
