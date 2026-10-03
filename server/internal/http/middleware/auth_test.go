package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	golangjwt "github.com/golang-jwt/jwt/v5"
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

// tokenAt mints a token with an explicit issued-at for floor-boundary tests.
func tokenAt(t *testing.T, issuedAt time.Time) string {
	t.Helper()
	claims := jwt.Claims{
		UserID: "user-1",
		RegisteredClaims: golangjwt.RegisteredClaims{
			ID:        "test-jti",
			ExpiresAt: golangjwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  golangjwt.NewNumericDate(issuedAt),
		},
	}
	tok, err := golangjwt.NewWithClaims(golangjwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return tok
}

// TestValidateTokenFloorSecondPrecision pins the one-second revocation
// window: the stored floor keeps microseconds, but the comparison truncates
// it, so a token from the floor's own second is valid while anything older
// is not. A zero floor (never reset) accepts everything unexpired.
func TestValidateTokenFloorSecondPrecision(t *testing.T) {
	floor := time.Now().Add(-time.Second).Truncate(time.Second).Add(500 * time.Millisecond)
	checker := stubChecker{state: SessionState{ValidAfter: floor, Role: "user"}}

	if _, _, ok := validateToken(testContext(), testSecret, nil, checker, tokenAt(t, floor.Add(-2*time.Second))); ok {
		t.Error("accepted a token issued two seconds before the floor")
	}
	if _, _, ok := validateToken(testContext(), testSecret, nil, checker, tokenAt(t, floor.Truncate(time.Second))); !ok {
		t.Error("rejected a token issued in the floor's own second")
	}
	if _, _, ok := validateToken(testContext(), testSecret, nil, checker, tokenAt(t, floor.Add(time.Second))); !ok {
		t.Error("rejected a token issued after the floor")
	}
	if _, _, ok := validateToken(testContext(), testSecret, nil, stubChecker{}, tokenAt(t, floor.Add(-time.Hour))); !ok {
		t.Error("rejected a token with a zero floor")
	}
}

func testContext() *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/me", nil)
	return c
}

// liveBlacklist dials the real Redis used by the compose stack (and the CI
// redis service) and skips when it is unreachable, following the
// requireEndpointDB convention of staying green without infrastructure.
func liveBlacklist(t *testing.T) *cache.Blacklist {
	t.Helper()
	addr := getenv("REDIS_ADDR", "127.0.0.1:6379")
	bl := cache.New(addr, getenv("REDIS_PASSWORD", ""))
	if err := bl.Ping(context.Background()); err != nil {
		t.Skip("redis unreachable, skipping live-blacklist test")
	}
	return bl
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// TestValidateTokenRejectsRevokedToken is the explicit counterpart to the
// outage test above: a JTI on the blacklist must be denied with the
// logged-out reason, not accepted and not confused with an outage.
func TestValidateTokenRejectsRevokedToken(t *testing.T) {
	bl := liveBlacklist(t)
	ctx := context.Background()

	tokenString := signedToken(t)
	claims, err := jwt.Validate(testSecret, tokenString)
	if err != nil {
		t.Fatalf("validate test token: %v", err)
	}
	if err := bl.Revoke(ctx, claims.ID, time.Minute); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	_, reason, ok := validateToken(testContext(), testSecret, bl, nil, tokenString)
	if ok {
		t.Fatal("validateToken accepted a revoked token")
	}
	if reason != "session was logged out, please sign in again" {
		t.Errorf("reason = %q, want the logged-out message", reason)
	}
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
