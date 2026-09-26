package users

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
)

// stubRoleReader is a one-method RoleReader, which is the point of the narrow
// interface: a test needs one method, not ten.
type stubRoleReader struct {
	role string
	err  error
	hits int
}

func (s *stubRoleReader) GetByID(context.Context, string) (User, error) {
	s.hits++
	if s.err != nil {
		return User{}, s.err
	}
	return User{Role: s.role}, nil
}

func roleContext(t *testing.T, userID string) *gin.Context {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	if userID != "" {
		c.Set(middleware.CtxUserID, userID)
	}
	return c
}

func TestCurrentRole(t *testing.T) {
	tests := []struct {
		name   string
		userID string
		stub   *stubRoleReader
		want   string
	}{
		{"owner", "u1", &stubRoleReader{role: RoleOwner}, RoleOwner},
		{"admin", "u1", &stubRoleReader{role: RoleAdmin}, RoleAdmin},
		{"user", "u1", &stubRoleReader{role: RoleUser}, RoleUser},
		// A lookup failure must deny rather than fall back to anything.
		{"lookup error denies", "u1", &stubRoleReader{err: errors.New("db down")}, ""},
		// No identity in context means no role, and no query either.
		{"anonymous", "", &stubRoleReader{role: RoleOwner}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := CurrentRole(tc.stub, roleContext(t, tc.userID)); got != tc.want {
				t.Fatalf("CurrentRole = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsModerator(t *testing.T) {
	tests := []struct {
		name   string
		userID string
		stub   *stubRoleReader
		want   bool
	}{
		{"admin moderates", "u1", &stubRoleReader{role: RoleAdmin}, true},
		{"owner moderates", "u1", &stubRoleReader{role: RoleOwner}, true},
		{"plain user does not", "u1", &stubRoleReader{role: RoleUser}, false},
		// The security-relevant case: a failed read must not grant privileges.
		{"lookup error does not grant", "u1", &stubRoleReader{err: errors.New("db down")}, false},
		{"anonymous does not", "", &stubRoleReader{role: RoleOwner}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsModerator(tc.stub, roleContext(t, tc.userID)); got != tc.want {
				t.Fatalf("IsModerator = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsModeratorSkipsLookupWhenAnonymous(t *testing.T) {
	stub := &stubRoleReader{role: RoleOwner}
	if IsModerator(stub, roleContext(t, "")) {
		t.Fatal("anonymous caller must not be a moderator")
	}
	if stub.hits != 0 {
		t.Fatalf("expected no database read for an anonymous caller, got %d", stub.hits)
	}
}

func TestRequireAdminAndRequireOwner(t *testing.T) {
	tests := []struct {
		name     string
		guard    func(RoleReader) gin.HandlerFunc
		stub     *stubRoleReader
		wantCode int
	}{
		{"admin passes RequireAdmin", RequireAdmin, &stubRoleReader{role: RoleAdmin}, http.StatusOK},
		{"owner passes RequireAdmin", RequireAdmin, &stubRoleReader{role: RoleOwner}, http.StatusOK},
		{"user fails RequireAdmin", RequireAdmin, &stubRoleReader{role: RoleUser}, http.StatusForbidden},
		{"lookup error fails RequireAdmin", RequireAdmin, &stubRoleReader{err: errors.New("db down")}, http.StatusForbidden},
		{"owner passes RequireOwner", RequireOwner, &stubRoleReader{role: RoleOwner}, http.StatusOK},
		{"admin fails RequireOwner", RequireOwner, &stubRoleReader{role: RoleAdmin}, http.StatusForbidden},
		{"user fails RequireOwner", RequireOwner, &stubRoleReader{role: RoleUser}, http.StatusForbidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			r := gin.New()
			r.Use(func(c *gin.Context) { c.Set(middleware.CtxUserID, "u1"); c.Next() })
			r.GET("/guarded", tc.guard(tc.stub), func(c *gin.Context) { c.Status(http.StatusOK) })

			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/guarded", nil))
			if w.Code != tc.wantCode {
				t.Fatalf("got %d, want %d", w.Code, tc.wantCode)
			}
		})
	}
}
