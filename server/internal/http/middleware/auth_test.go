package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/platform/cache"
	"github.com/jojianya/sweetspot247-backend/pkg/jwt"
)

const testSecret = "test-secret-for-blacklist-fail-closed"

func init() { gin.SetMode(gin.TestMode) }

// unreachableBlacklist points a Blacklist at a port nothing listens on, so every
// IsRevoked call errors the way it would during a real Redis outage. Port 1 on
// loopback is reserved, so the dial fails fast instead of hanging.
func unreachableBlacklist() *cache.Blacklist {
	return cache.New("127.0.0.1:1", "")
}

func signedToken(t *testing.T) string {
	t.Helper()
	tok, err := jwt.Generate(testSecret, "user-1", time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return tok
}

func testContext() *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/me", nil)
	return c
}

// TestValidateTokenFailsClosedWhenBlacklistUnavailable is the regression test for
// the audit finding that a logged-out token was accepted again while Redis was
// down. IsRevoked returns an error on outage; the request must be denied rather
// than let through on the assumption that the token is still valid.
func TestValidateTokenFailsClosedWhenBlacklistUnavailable(t *testing.T) {
	_, reason, ok := validateToken(testContext(), testSecret, unreachableBlacklist(), nil, signedToken(t))

	if ok {
		t.Fatal("validateToken accepted a token while the blacklist was unreachable; " +
			"this resurrects logged-out sessions during a Redis outage")
	}
	if reason == "" {
		t.Error("expected a non-empty reason for the denial, got an empty string")
	}
}

// TestValidateTokenAllowsWhenNoBlacklistConfigured proves the fix is not simply
// "always deny": with no revocation store configured the token is still
// signature-checked and accepted.
func TestValidateTokenAllowsWhenNoBlacklistConfigured(t *testing.T) {
	claims, reason, ok := validateToken(testContext(), testSecret, nil, nil, signedToken(t))
	if !ok {
		t.Fatalf("validateToken rejected a valid token with no blacklist configured (reason: %s)", reason)
	}
	if claims == nil {
		t.Error("expected claims to be returned on success")
	}
}

// TestValidateTokenRejectsBadSignature confirms the deny path is specific to the
// blacklist failure and is not masking signature validation.
func TestValidateTokenRejectsBadSignature(t *testing.T) {
	if _, _, ok := validateToken(testContext(), "a-different-secret", nil, nil, signedToken(t)); ok {
		t.Error("validateToken accepted a token signed with a different secret")
	}
}

// stubChecker is a SessionChecker with a fixed floor, role, or error.
type stubChecker struct {
	state SessionState
	err   error
}

func (s stubChecker) CheckSession(context.Context, string) (SessionState, error) {
	return s.state, s.err
}

func TestValidateTokenRejectsSessionBeforeFloor(t *testing.T) {
	checker := stubChecker{state: SessionState{
		ValidAfter: time.Now().Add(time.Minute),
		Role:       "user",
	}}
	_, reason, ok := validateToken(testContext(), testSecret, nil, checker, signedToken(t))
	if ok {
		t.Fatal("validateToken accepted a token issued before sessions_valid_after")
	}
	if reason == "" {
		t.Error("expected a non-empty reason for the denial, got an empty string")
	}
}

func TestValidateTokenAcceptsSessionAfterFloor(t *testing.T) {
	c := testContext()
	checker := stubChecker{state: SessionState{
		ValidAfter: time.Now().Add(-time.Minute),
		Role:       "admin",
	}}
	_, _, ok := validateToken(c, testSecret, nil, checker, signedToken(t))
	if !ok {
		t.Fatal("validateToken rejected a token issued after sessions_valid_after")
	}
	if got := c.GetString(CtxRole); got != "admin" {
		t.Errorf("expected cached role %q, got %q", "admin", got)
	}
}

func TestValidateTokenFailsClosedWhenCheckerErrors(t *testing.T) {
	checker := stubChecker{err: errors.New("db down")}
	_, reason, ok := validateToken(testContext(), testSecret, nil, checker, signedToken(t))
	if ok {
		t.Fatal("validateToken accepted a token while the session check errored")
	}
	if reason == "" {
		t.Error("expected a non-empty reason for the denial, got an empty string")
	}
}
