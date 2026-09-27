package endpointtest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	users "github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/pkg/password"
)

// Regression tests for the rune/byte mismatch on password length.
//
// The DTO's binding tag counted runes while bcrypt counts bytes, so a
// non-ASCII password could pass validation and then be refused by bcrypt. That
// surfaced as HTTP 500 on user input. These assert the status is 400 and that no
// user is created.

// countingUserService records how far a registration got, so a test can assert
// that a rejected password was refused before any user was created.
type countingUserService struct {
	users.Service
	CreateCalls int
}

func (c *countingUserService) Create(ctx context.Context, email, hash, username string) (users.User, error) {
	c.CreateCalls++
	return c.Service.Create(ctx, email, hash, username)
}

// registerWithPassword posts a registration with the given password. The
// username is derived from the email but padded past the 3-rune minimum, so a
// failure here is always about the password and never the username.
func registerWithPassword(t *testing.T, r *gin.Engine, email, pw string) *httptest.ResponseRecorder {
	t.Helper()
	username := "user-" + strings.SplitN(email, "@", 2)[0]
	return doJSON(t, r, http.MethodPost, "/auth/register",
		`{"email":"`+email+`","password":"`+pw+`","username":"`+username+`"}`, nil)
}

func TestRegisterPasswordLengthIsCheckedInBytes(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantCode int
	}{
		// Comfortably inside both limits.
		{"ascii typical", "password123", http.StatusCreated},
		{"ascii at the byte limit", strings.Repeat("a", password.MaxLen), http.StatusCreated},
		{"ascii one byte over", strings.Repeat("a", password.MaxLen+1), http.StatusBadRequest},
		{"ascii under the rune minimum", "short", http.StatusBadRequest},
		{"ascii at the rune minimum", strings.Repeat("a", password.MinRunes), http.StatusCreated},

		// These are the regression cases: within 72 runes, over 72 bytes. They
		// returned 500 before the fix.
		{"24 three-byte runes is exactly the limit", strings.Repeat("漢", 24), http.StatusCreated},
		{"25 three-byte runes over the byte limit", strings.Repeat("漢", 25), http.StatusBadRequest},
		{"36 two-byte runes is exactly the limit", strings.Repeat("é", 36), http.StatusCreated},
		{"37 two-byte runes over the byte limit", strings.Repeat("é", 37), http.StatusBadRequest},
		{"eight emoji within the limit", strings.Repeat("\U0001F600", 8), http.StatusCreated},
		{"twenty emoji over the byte limit", strings.Repeat("\U0001F600", 20), http.StatusBadRequest},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			usersSvc := newUsersSvc()
			// Wrap the mock to observe whether registration got as far as
			// creating a user. The mock discards what Create returns, so this is
			// the only way to tell "rejected before creating" from "rejected
			// somewhere else".
			counted := &countingUserService{Service: usersSvc}
			r, _ := setupFeaturesRouter(t, &stubPinRepo{}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{}, counted)

			w := registerWithPassword(t, r, fmt.Sprintf("u%d@example.com", i), tc.password)
			if w.Code != tc.wantCode {
				t.Fatalf("password of %d runes / %d bytes: got HTTP %d, want %d (%s)",
					len([]rune(tc.password)), len(tc.password), w.Code, tc.wantCode, w.Body.String())
			}

			if tc.wantCode == http.StatusBadRequest && counted.CreateCalls > 0 {
				t.Fatalf("rejected registration still reached user creation %d time(s)", counted.CreateCalls)
			}
			if tc.wantCode == http.StatusCreated && counted.CreateCalls != 1 {
				t.Fatalf("accepted registration reached user creation %d times, want 1", counted.CreateCalls)
			}
		})
	}
}

// The error must be a 4xx, not a 500: this is client input, and a 500 sends
// operators chasing a server fault that does not exist.
func TestOverLongPasswordIsNotAServerError(t *testing.T) {
	usersSvc := newUsersSvc()
	r, _ := setupFeaturesRouter(t, &stubPinRepo{}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{}, usersSvc)

	w := registerWithPassword(t, r, "over@example.com", strings.Repeat("漢", 40))
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("over-long password returned 500, got body: %s", w.Body.String())
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got HTTP %d, want 400 (%s)", w.Code, w.Body.String())
	}
	// The message should be actionable, not the generic binding error.
	if !strings.Contains(w.Body.String(), "72") {
		t.Fatalf("error should mention the 72-byte limit, got: %s", w.Body.String())
	}
}

// Too short is the other direction, and must stay a 400.
func TestShortPasswordIsRejected(t *testing.T) {
	usersSvc := newUsersSvc()
	r, _ := setupFeaturesRouter(t, &stubPinRepo{}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{}, usersSvc)

	w := registerWithPassword(t, r, "short@example.com", "short")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got HTTP %d, want 400 (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "8") {
		t.Fatalf("error should mention the 8-character minimum, got: %s", w.Body.String())
	}
}

// A password that passes validation must hash and verify intact, at the byte
// limit included. Without this, tightening the check could have locked out the
// boundary or started comparing only a prefix.
//
// This exercises the password package rather than the HTTP layer: the mock user
// service does not persist new users, so an end-to-end login here would test
// the harness rather than the credential.
func TestPasswordAtByteLimitHashesAndVerifies(t *testing.T) {
	pw := strings.Repeat("漢", 24) // exactly 72 bytes
	if len(pw) != password.MaxLen {
		t.Fatalf("test fixture is %d bytes, want %d", len(pw), password.MaxLen)
	}
	if err := password.Validate(pw); err != nil {
		t.Fatalf("a %d-byte password should be accepted: %v", len(pw), err)
	}

	hash, err := password.Hash(pw)
	if err != nil {
		t.Fatalf("Hash at the byte limit: %v", err)
	}
	if !password.Verify(pw, hash) {
		t.Fatal("a 72-byte password should verify against its own hash")
	}

	// The stored credential must be the whole password, not a 72-byte prefix.
	// This is the truncation scenario: a different tail must not authenticate.
	wrongTail := strings.Repeat("漢", 23) + "字"
	if password.Verify(wrongTail, hash) {
		t.Fatal("a password sharing a 72-byte prefix authenticated")
	}
}
