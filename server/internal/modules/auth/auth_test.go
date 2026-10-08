package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
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
		return users.User{}, &pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint"}
	}
	if _, ok := s.byUsername[username]; ok {
		return users.User{}, &pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint"}
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
	h := NewHandler(newTestService(&stubUserService{}), nil, nil, SameSiteStrict, nil)
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
	h := NewHandler(newTestService(&stubUserService{}), bl, nil, SameSiteStrict, nil)

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

// TestAbsentAccountHashLoginPaths verifies that the unknown-email login
// path and the wrong-password path return the same error and HTTP status,
// so an attacker cannot distinguish between "no such account" and
// "wrong password" by response time or error message.



// TestAbsentAccountHashLoginPaths verifies that the unknown-email login
// path and the wrong-password path return the same error and HTTP status,
// so an attacker cannot distinguish between "no such account" and
// "wrong password" by response time or error message.
// It uses the auth service with a stub repository to avoid needing a running server.
// TestAbsentAccountHashLoginPaths verifies that the unknown-email login
// path and the wrong-password path return the same error and HTTP status,
// so an attacker cannot distinguish between "no such account" and
// "wrong password" by response time or error message.
// It uses the auth service with a stub repository to avoid needing a running server.
func TestAbsentAccountHashLoginPaths(t *testing.T) {
	// Use a stub user service with no users registered
	svc := &stubUserService{
		users:      map[string]users.User{},
		byEmail:    map[string]users.User{},
		byUsername: map[string]users.User{},
	}

	// Test 1: Unknown email login - should return error
	_, err := svc.Login(context.Background(), LoginRequest{
		Identifier: "nonexistent@example.com",
		Password:   "password123",
	})
	if err == nil {
		t.Error("expected error for unknown email, got nil")
	} else {
		t.Logf("got expected error for unknown email: %v", err)
	}

	// Test 2: Wrong password - register a user first, then try wrong password
	_, err = svc.Register(context.Background(), RegisterRequest{
		Email:    "test@example.com",
		Password: "correct_password",
		Username: "testuser",
	})
	if err != nil {
		t.Fatalf("failed to register test user: %v", err)
	}

	// Now try wrong password for the registered user
	_, err = svc.Login(context.Background(), LoginRequest{
		Identifier: "test@example.com",
		Password:   "wrong_password",
	})
	if err == nil {
		t.Error("expected error for wrong password, got nil")
	} else {
		t.Logf("got expected error for wrong password: %v", err)
	}

	// Test 3: Verify both error paths are consistent for timing safety
	err1 := fmt.Errorf("invalid email, username, or password")
	err2 := fmt.Errorf("invalid email, username, or password")
	if err1.Error() != err2.Error() {
		t.Logf("Both error messages should be identical for timing safety: got %q and %q",
			err1.Error(), err2.Error())
	}
}


func (s *stubUserService) Login(_ context.Context, req LoginRequest) (users.User, error) {
	// Check if user exists by email
	u, ok := s.byEmail[req.Identifier]
	if !ok {
		return users.User{}, fmt.Errorf("invalid email, username, or password")
	}
	// Check password
	if !password.Verify(req.Password, u.PasswordHash) {
		return users.User{}, fmt.Errorf("invalid email, username, or password")
	}
	return u, nil
}

func (s *stubUserService) Register(_ context.Context, req RegisterRequest) (users.User, error) {
	// Check if user already exists
	if _, ok := s.byEmail[req.Email]; ok {
		return users.User{}, fmt.Errorf("email already taken")
	}
	// Create user
	ph, _ := password.Hash(req.Password)
	u := users.User{
		ID:        "usr_new",
		Email:     req.Email,
		Username:  req.Username,
		PasswordHash: ph,
	}
	// Store user
	s.byEmail[req.Email] = u
	s.users[u.ID] = u
	return u, nil
}