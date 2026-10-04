package endpointtest

// HTTP-level proof that a blacklisted session is rejected end to end: the
// token verifies and works, its JTI lands on the live Redis blacklist, and
// the same request then answers 401 with the logged-out message.
//
// Gated like the other endpoint DB tests (ENDPOINT_TEST_DB + DB_PASSWORD, so
// it runs in the endpoint CI step and skips in a plain `go test ./...`), and
// additionally skipped when Redis is unreachable. No Postgres data is used;
// the "DB" prefix keeps the CI -run filter convention.

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jojianya/sweetspot247-backend/internal/modules/auth"
	users "github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/internal/platform/cache"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
	"github.com/jojianya/sweetspot247-backend/pkg/jwt"
)

func redisAddr() (addr, password string) {
	addr = "127.0.0.1:6379"
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		addr = v
	}
	return addr, os.Getenv("REDIS_PASSWORD")
}

func TestDBAuthRevokedTokenDenied(t *testing.T) {
	if os.Getenv("ENDPOINT_TEST_DB") == "" || os.Getenv("DB_PASSWORD") == "" {
		t.Skip("ENDPOINT_TEST_DB/DB_PASSWORD not set; runs with the endpoint suite")
	}
	addr, password := redisAddr()
	bl := cache.New(addr, password)
	if err := bl.Ping(context.Background()); err != nil {
		t.Skip("redis unreachable, skipping live-blacklist test")
	}

	usersSvc := &mockUserService{
		users: map[string]users.User{
			testUUID2: {ID: testUUID2, Email: "b@example.com", Username: "bob", Role: users.RoleUser},
		},
	}
	r := setupRouter(usersSvc, &mockReportRepo{}, &mockFavoriteRepo{}, bl, storage.NewLocal(t.TempDir(), "http://test.local"), auth.SameSiteStrict)

	tokenString := newToken(t, testUUID2)
	bearer := map[string]string{"Authorization": "Bearer " + tokenString}

	// PATCH /users/me is AuthRequired: pre-revoke the request reaches the
	// handler (400 for the empty body), proving the token verifies.
	if w := doJSON(t, r, http.MethodPatch, "/users/me", "", bearer); w.Code == http.StatusUnauthorized {
		t.Fatalf("pre-revoke: got 401 (%s), want the handler to run", w.Body.String())
	}

	claims, err := jwt.Validate(testSecret, tokenString)
	if err != nil {
		t.Fatalf("validate test token: %v", err)
	}
	if err := bl.Revoke(context.Background(), claims.ID, time.Minute); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	w := doJSON(t, r, http.MethodPatch, "/users/me", "", bearer)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("post-revoke: got %d (%s), want 401", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	if msg, _ := body["error"].(string); !strings.Contains(msg, "logged out") {
		t.Errorf("post-revoke error = %q, want the logged-out message", body["error"])
	}
}
