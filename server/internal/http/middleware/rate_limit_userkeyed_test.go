package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func testContextWithUser(t *testing.T, userID string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/test", nil)
	if userID != "" {
		c.Set(CtxUserID, userID)
	}
	return c, w
}

// TestMiddlewareUserKeyedAuthenticatedWithinBudget verifies an authenticated
// caller stays within the per-user budget.
func TestMiddlewareUserKeyedAuthenticatedWithinBudget(t *testing.T) {
	l := New(5, time.Minute)
	mw := l.MiddlewareUserKeyed("test:", true)

	c, w := testContextWithUser(t, "user-1")
	for i := 0; i < 5; i++ {
		mw(c)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: want 200, got %d", i+1, w.Code)
		}
		c, w = testContextWithUser(t, "user-1")
	}
}

// TestMiddlewareUserKeyedAuthenticatedOverBudget verifies an authenticated
// caller exceeding the budget gets 429.
func TestMiddlewareUserKeyedAuthenticatedOverBudget(t *testing.T) {
	l := New(2, time.Minute)
	mw := l.MiddlewareUserKeyed("test:", true)

	c, w := testContextWithUser(t, "user-1")
	mw(c)
	if w.Code != http.StatusOK {
		t.Fatalf("request 1: want 200, got %d", w.Code)
	}

	c, w = testContextWithUser(t, "user-1")
	mw(c)
	if w.Code != http.StatusOK {
		t.Fatalf("request 2: want 200, got %d", w.Code)
	}

	c, w = testContextWithUser(t, "user-1")
	mw(c)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("request 3: want 429, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "too many requests") {
		t.Errorf("429 response body = %q, want 'too many requests'", w.Body.String())
	}
}

// TestMiddlewareUserKeyedAnonymousRequireAuthTrue verifies an anonymous
// caller with requireAuth=true gets 401.
func TestMiddlewareUserKeyedAnonymousRequireAuthTrue(t *testing.T) {
	l := New(5, time.Minute)
	mw := l.MiddlewareUserKeyed("test:", true)

	c, w := testContextWithUser(t, "")
	mw(c)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous with requireAuth=true: want 401, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "auth required") {
		t.Errorf("401 response body = %q, want 'auth required'", w.Body.String())
	}
}

// TestMiddlewareUserKeyedAnonymousRequireAuthFalse verifies an anonymous
// caller with requireAuth=false passes through without consuming budget.
func TestMiddlewareUserKeyedAnonymousRequireAuthFalse(t *testing.T) {
	l := New(1, time.Minute)
	mw := l.MiddlewareUserKeyed("test:", false)

	// First anonymous request passes
	c, w := testContextWithUser(t, "")
	mw(c)
	if w.Code != http.StatusOK {
		t.Fatalf("first anonymous: want 200, got %d", w.Code)
	}

	// Second anonymous request also passes (no shared budget key)
	c, w = testContextWithUser(t, "")
	mw(c)
	if w.Code != http.StatusOK {
		t.Fatalf("second anonymous: want 200, got %d", w.Code)
	}

	// Authenticated caller on the same limiter instance gets own budget
	c, w = testContextWithUser(t, "user-1")
	mw(c)
	if w.Code != http.StatusOK {
		t.Fatalf("authenticated: want 200, got %d", w.Code)
	}
}

// TestMiddlewareUserKeyedSeparateUsersSeparateBudgets verifies different
// authenticated users get separate budgets.
func TestMiddlewareUserKeyedSeparateUsersSeparateBudgets(t *testing.T) {
	l := New(2, time.Minute)
	mw := l.MiddlewareUserKeyed("test:", true)

	// user-1 uses 2 requests
	c, w := testContextWithUser(t, "user-1")
	mw(c)
	c, w = testContextWithUser(t, "user-1")
	mw(c)
	if w.Code != http.StatusOK {
		t.Fatalf("user-1 request 2: want 200, got %d", w.Code)
	}

	// user-1 third request should be 429
	c, w = testContextWithUser(t, "user-1")
	mw(c)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("user-1 request 3: want 429, got %d", w.Code)
	}

	// user-2 should still have full budget
	c, w = testContextWithUser(t, "user-2")
	mw(c)
	if w.Code != http.StatusOK {
		t.Fatalf("user-2 request 1: want 200, got %d", w.Code)
	}
}
