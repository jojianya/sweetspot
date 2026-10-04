package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	users "github.com/jojianya/sweetspot247-backend/internal/modules/user"
)

// registerUsernameHandler serves only /auth/register with the given stub, so a
// character rule failure cannot be masked by the per-IP rate limiter.
func registerUsernameHandler(t *testing.T, svc *stubUserService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(NewService(svc, "test-secret"), nil, nil, SameSiteStrict, nil)
	r.POST("/auth/register", h.Register)
	return r
}

// postRegisterBody marshals the request properly. Building the JSON by hand
// would be wrong for exactly the inputs under test: a username containing a
// quote or a backslash has to be escaped, and an unescaped one produces a
// malformed body that fails in ShouldBindJSON before the username rule is ever
// consulted — testing the harness instead of the handler.
func postRegisterBody(t *testing.T, r *gin.Engine, username string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"email":    "a@example.com",
		"password": "password123",
		"username": username,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func errorMessageOf(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", w.Body.String(), err)
	}
	return body["error"]
}

// TestRegisterRejectsInvalidUsernameCharacters covers the registration half of
// the username allowlist. The handler must answer 400 with the rule's own
// message: the character check sits after ShouldBindJSON, and without it these
// would fall through to the 500 branch as an unmapped error.
func TestRegisterRejectsInvalidUsernameCharacters(t *testing.T) {
	r := registerUsernameHandler(t, &stubUserService{
		byEmail:    map[string]users.User{},
		byUsername: map[string]users.User{},
	})

	bad := []struct {
		username string
		why      string
	}{
		{"alice smith", "space"},
		{"<script>", "angle brackets"},
		{`alice"`, "double quote"},
		{"аlice", "Cyrillic a homoglyph"},
		{"alice/bob", "slash"},
		{"../../etc", "path traversal"},
		{"café", "non-ASCII letter"},
	}
	for _, tc := range bad {
		t.Run(tc.why, func(t *testing.T) {
			w := postRegisterBody(t, r, tc.username)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("username %q: status = %d, want 400 (%s)", tc.username, w.Code, w.Body.String())
			}
			if msg := errorMessageOf(t, w); msg != "username may only contain letters, numbers, and _ . -" {
				t.Errorf("username %q: error = %q", tc.username, msg)
			}
		})
	}
}

// Registration already rejected short and long usernames through the DTO's
// `min=3,max=30` binding tags, and that check runs first inside ShouldBindJSON.
// So a length problem here is still a 400, just carrying gin's validator
// wording rather than the rule's — pre-existing behaviour the character rule
// does not change. What matters is that it is a 400 and never a 500.
func TestRegisterRejectsOutOfRangeLength(t *testing.T) {
	r := registerUsernameHandler(t, &stubUserService{
		byEmail:    map[string]users.User{},
		byUsername: map[string]users.User{},
	})
	for _, name := range []string{"ab", strings.Repeat("a", 31)} {
		w := postRegisterBody(t, r, name)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("username of length %d: status = %d, want 400 (%s)", len(name), w.Code, w.Body.String())
		}
	}
}

// A valid username still registers, so the rule does not reject good input.
func TestRegisterAcceptsValidUsername(t *testing.T) {
	svc := &stubUserService{byEmail: map[string]users.User{}, byUsername: map[string]users.User{}}
	r := registerUsernameHandler(t, svc)

	for _, name := range []string{"alice", "user_name", "user.name", "user-name", "User99"} {
		w := postRegisterBody(t, r, name)
		if w.Code != http.StatusCreated {
			t.Fatalf("username %q: status = %d, want 201 (%s)", name, w.Code, w.Body.String())
		}
	}
}
