package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/pkg/password"
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

func (s *stubUserService) GetByID(_ context.Context, id string) (users.User, error) {
	u, ok := s.users[id]
	if !ok {
		return users.User{}, users.ErrNotFound
	}
	return u, nil
}

func (s *stubUserService) UpdateRole(context.Context, string, string, string) (users.User, error) {
	return users.User{}, nil
}

func newTestService(stub *stubUserService) Service {
	return NewService(stub, "test-secret")
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
		Email:    "a@example.com",
		Password: "password123",
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
		Email:    "a@example.com",
		Password: "wrong-password",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginUnknownEmail(t *testing.T) {
	svc := newTestService(&stubUserService{})
	_, _, err := svc.Login(context.Background(), LoginRequest{
		Email:    "nobody@example.com",
		Password: "password123",
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
