package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/internal/platform/cache"
	"github.com/jojianya/sweetspot247-backend/pkg/jwt"
	"github.com/jojianya/sweetspot247-backend/pkg/password"
	"golang.org/x/crypto/bcrypt"
)

type stubUserService struct {
	users      map[string]users.User
	byEmail    map[string]users.User
	byUsername map[string]users.User
}

func (s *stubUserService) Create(_ context.Context, email, passwordHash, username string) (users.User, error) {
	if _, ok := s.byEmail[email]; ok {
		return users.User{}, users.ErrDuplicate
	}
	if _, ok := s.byUsername[username]; ok {
		return users.User{}, users.ErrDuplicate
	}
	u := users.User{ID: "usr_new", Email: email, Username: username, Role: users.RoleUser, PasswordHash: passwordHash}
	return u, nil
}

func (s *stubUserService) GetByEmail(_ context.Context, email string) (users.User, error) {
	u, ok := s.byEmail[email]
	if !ok {
		return users.User{}, users.ErrNotFound
	}
	return u, nil
}

func (s *stubUserService) GetByUsername(_ context.Context, username string) (users.User, error) {
	u, ok := s.byUsername[username]
	if !ok {
		return users.User{}, users.ErrNotFound
	}
	return u, nil
}

func (s *stubUserService) GetByLogin(_ context.Context, identifier string) (users.User, error) {
	if u, ok := s.byEmail[identifier]; ok {
		return u, nil
	}
	if u, ok := s.byUsername[identifier]; ok {
		return u, nil
	}
	return users.User{}, users.ErrNotFound
}

func (s *stubUserService) GetByID(_ context.Context, id string) (users.User, error) {
	u, ok := s.users[id]
	if !ok {
		return users.User{}, users.ErrNotFound
	}
	return u, nil
}

func (s *stubUserService) CheckSession(_ context.Context, id string) (middleware.SessionState, error) {
	u, ok := s.users[id]
	if !ok {
		return middleware.SessionState{}, users.ErrNotFound
	}
	return middleware.SessionState{Role: u.Role}, nil
}

func (s *stubUserService) UpdateRole(context.Context, string, string, string) (users.User, error) {
	return users.User{}, nil
}

func (s *stubUserService) SearchUsers(context.Context, string, int, int) ([]users.User, int, error) {
	return []users.User{}, 0, nil
}

func (s *stubUserService) ListUsers(_ context.Context, _ int, _ int) ([]users.User, int, error) {
	return []users.User{}, 0, nil
}

func (s *stubUserService) CountOwners(_ context.Context) (int, error) {
	return 0, nil
}

func (s *stubUserService) CountUsers(_ context.Context) (int, error) {
	return 0, nil
}

func (s *stubUserService) UpdateProfile(context.Context, string, users.UpdateProfilePatch) (users.User, error) {
	return users.User{}, nil
}

func newTestService(stub *stubUserService) Service {
	return NewService(stub, "test-secret")
}

func TestMeServiceErrorReturns500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(newTestService(&stubUserService{}), nil, nil, nil, SameSiteStrict, nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	c.Set("user_id", "missing")
	h.Me(c)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

func TestRegisterSuccess(t *testing.T) {
	svc := newTestService(&stubUserService{})
	u, token, err := svc.Register(context.Background(), RegisterRequest{
		Email:    "new@example.com",
		Password: "password123",
		Username: "newuser",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID == "" {
		t.Fatal("expected a user id")
	}
	if token == "" {
		t.Fatal("expected a token")
	}
}

func TestRegisterEmailTaken(t *testing.T) {
	svc := newTestService(&stubUserService{
		byEmail: map[string]users.User{"taken@example.com": {Email: "taken@example.com"}},
	})
	_, _, err := svc.Register(context.Background(), RegisterRequest{
		Email:    "taken@example.com",
		Password: "password123",
		Username: "newuser",
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestRegisterUsernameTaken(t *testing.T) {
	svc := newTestService(&stubUserService{
		byUsername: map[string]users.User{"taken": {Username: "taken"}},
	})
	_, _, err := svc.Register(context.Background(), RegisterRequest{
		Email:    "new@example.com",
		Password: "password123",
		Username: "taken",
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestLoginSuccess(t *testing.T) {
	hash, err := password.Hash("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	svc := newTestService(&stubUserService{
		byEmail: map[string]users.User{
			"a@example.com": {ID: "u1", Email: "a@example.com", Role: users.RoleUser, PasswordHash: hash},
		},
	})
	u, token, err := svc.Login(context.Background(), LoginRequest{
		Identifier: "a@example.com",
		Password:   "password123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID != "u1" {
		t.Fatalf("expected user u1, got %s", u.ID)
	}
	if token == "" {
		t.Fatal("expected a token")
	}
}

func TestLoginByUsername(t *testing.T) {
	hash, err := password.Hash("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	svc := newTestService(&stubUserService{
		byUsername: map[string]users.User{
			"alice": {ID: "u1", Email: "a@example.com", Username: "alice", Role: users.RoleUser, PasswordHash: hash},
		},
	})
	u, _, err := svc.Login(context.Background(), LoginRequest{
		Identifier: "alice",
		Password:   "password123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID != "u1" {
		t.Fatalf("expected user u1, got %s", u.ID)
	}
}

func TestLoginInvalidPassword(t *testing.T) {
	hash, err := password.Hash("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	svc := newTestService(&stubUserService{
		byEmail: map[string]users.User{
			"a@example.com": {ID: "u1", Email: "a@example.com", PasswordHash: hash},
		},
	})
	_, _, err = svc.Login(context.Background(), LoginRequest{
		Identifier: "a@example.com",
		Password:   "wrong-password",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginUnknownIdentifier(t *testing.T) {
	svc := newTestService(&stubUserService{})
	_, _, err := svc.Login(context.Background(), LoginRequest{
		Identifier: "nobody@example.com",
		Password:   "password123",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestRoleByID(t *testing.T) {
	svc := newTestService(&stubUserService{
		users: map[string]users.User{"u1": {ID: "u1", Role: users.RoleOwner}},
	})
	role, err := svc.Role(context.Background(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if role != users.RoleOwner {
		t.Fatalf("expected role %q, got %q", users.RoleOwner, role)
	}
}

func TestLogoutSucceedsWhenRedisIsDown(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Simulate a Redis outage: use a real Blacklist pointing at a closed port.
	// Revoke will fail with a connection error, which is what we're testing.
	bl := cache.New("127.0.0.1:1", "")
	h := NewHandler(newTestService(&stubUserService{}), bl, nil, nil, SameSiteStrict, nil)

	// Create a valid JWT so the handler can extract claims.
	token, err := jwt.Generate("test-secret", "u1", time.Hour)
	if err != nil {
		t.Fatalf("jwt.Generate: %v", err)
	}
	claims, err := jwt.Validate("test-secret", token)
	if err != nil {
		t.Fatalf("jwt.Validate: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	c.Set(middleware.CtxJWTClaims, claims)

	h.Logout(c)

	// The logout must succeed (200) even when Redis is down.
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}

	// The cookie must be cleared.
	cookies := w.Header().Values("Set-Cookie")
	found := false
	for _, line := range cookies {
		if strings.Contains(line, "session_token=;") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected session_token cookie to be cleared, got %v", cookies)
	}
}

// TestAbsentAccountHashCostMatchesRealHash verifies that the
// absent account hash cost matches the cost of a freshly generated real
// password hash. This guarantees the timing-safe login guard works
// correctly: if the costs drifted apart, an attacker could exploit the
// timing difference to enumerate accounts.
func TestAbsentAccountHashCostMatchesRealHash(t *testing.T) {
	realHash, err := password.GenerateAbsentAccountHash()
	if err != nil {
		t.Fatalf("failed to generate real hash: %v", err)
	}
	absentCost, _ := bcrypt.Cost([]byte(password.GetAbsentAccountHash()))
	realCost, _ := bcrypt.Cost([]byte(realHash))
	if absentCost != realCost {
		t.Errorf("absentAccountHash cost = %d, real hash cost = %d; they must match",
			absentCost, realCost)
	}
	if absentCost < 12 {
		t.Errorf("cost = %d, want at least 12", absentCost)
	}
}

// TestAbsentAccountHashLoginPaths verifies that the unknown-email login path
// and the wrong-password path return the same error, so an attacker cannot
// distinguish between "no such account" and "wrong password" by response
// message. It exercises the real auth service with a stub user service (fake
// dependency, real thing under test).
func TestAbsentAccountHashLoginPaths(t *testing.T) {
	hash, err := password.Hash("correct_password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	svc := newTestService(&stubUserService{
		users: map[string]users.User{
			"usr_1": {ID: "usr_1", Email: "test@example.com", Username: "testuser", PasswordHash: hash},
		},
		byEmail: map[string]users.User{
			"test@example.com": {ID: "usr_1", Email: "test@example.com", Username: "testuser", PasswordHash: hash},
		},
		byUsername: map[string]users.User{},
	})

	_, _, unknownErr := svc.Login(context.Background(), LoginRequest{
		Identifier: "nonexistent@example.com",
		Password:   "password123",
	})
	if !errors.Is(unknownErr, ErrInvalidCredentials) {
		t.Fatalf("unknown email err = %v, want ErrInvalidCredentials", unknownErr)
	}

	_, _, wrongPwErr := svc.Login(context.Background(), LoginRequest{
		Identifier: "test@example.com",
		Password:   "wrong_password",
	})
	if !errors.Is(wrongPwErr, ErrInvalidCredentials) {
		t.Fatalf("wrong password err = %v, want ErrInvalidCredentials", wrongPwErr)
	}

	if unknownErr.Error() != wrongPwErr.Error() {
		t.Fatalf("login errors differ: %q vs %q; they must be identical", unknownErr, wrongPwErr)
	}
}

// TestAbsentAccountHashGet returns the absent account hash and validates it.
// It fails if the hash is empty or bcrypt.Cost can't read it.
func TestAbsentAccountHashGet(t *testing.T) {
	hash := password.GetAbsentAccountHash()
	if hash == "" {
		t.Fatal("GetAbsentAccountHash returned empty string")
	}
	cost, _ := bcrypt.Cost([]byte(hash))
	if cost < 0 {
		t.Fatalf("bcrypt.Cost returned %d, expected >= 0", cost)
	}
	if cost < 12 {
		t.Errorf("bcrypt.Cost = %d, want >= 12", cost)
	}
}
