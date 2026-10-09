package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/pkg/jwt"
	"github.com/jojianya/sweetspot247-backend/pkg/password"
)

const tokenExpiry = 30 * 24 * time.Hour

// AbsentAccountHash() is a bcrypt hash of a value no one can submit.
// It is generated at init time using the same cost as real passwords (via
// server/pkg/password.init()), so a missing account costs the same as a wrong
// password and the two cannot be told apart by response time. The hash is
// produced by pkg/password.AbsentAccountHash, which is generated once at
// package initialization using bcryptCost = 12.

// AbsentAccountHash returns the absent account hash, lazily initialized
// via sync.Once in pkg/password. This ensures the hash is always available
// when needed and prevents a read before the first Once.Do.
// AbsentAccountHash returns the absent account hash, lazily initialized
// via sync.Once in pkg/password. This ensures the hash is always available
// when needed and prevents a read before the first Once.Do.
func AbsentAccountHash() string {
	return password.GetAbsentAccountHash()
}

// Service interface and implementation.
type Service interface {
	Register(ctx context.Context, req RegisterRequest) (users.User, string, error)
	Login(ctx context.Context, req LoginRequest) (users.User, string, error)
	Role(ctx context.Context, userID string) (string, error)
}

type service struct {
	users     users.Service
	jwtSecret string
}

func NewService(userSvc users.Service, jwtSecret string) Service {
	return &service{users: userSvc, jwtSecret: jwtSecret}
}

func (s *service) Register(ctx context.Context, req RegisterRequest) (users.User, string, error) {
	// Checked here rather than via the DTO's binding tag: the tag counts runes
	// and bcrypt counts bytes, so only this check reflects what bcrypt will
	// accept. Wrapped in ErrInvalidPassword so the handler answers 400.
	if err := password.Validate(req.Password); err != nil {
		return users.User{}, "", fmt.Errorf("%w: %w", ErrInvalidPassword, err)
	}

	hash, err := password.Hash(req.Password)
	if err != nil {
		return users.User{}, "", err
	}

	u, err := s.users.Create(ctx, req.Email, hash, req.Username)
	if err != nil {
		if errors.Is(err, users.ErrDuplicate) {
			return users.User{}, "", ErrConflict
		}
		return users.User{}, "", err
	}

	token, err := jwt.Generate(s.jwtSecret, u.ID, tokenExpiry)
	if err != nil {
		return users.User{}, "", err
	}
	return u, token, nil
}

func (s *service) Login(ctx context.Context, req LoginRequest) (users.User, string, error) {
	u, err := s.users.GetByLogin(ctx, req.Identifier)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			// Spend the same time as a wrong password would, so an absent
			// account is not distinguishable by how fast the 401 arrives.
			password.Verify(req.Password, AbsentAccountHash())
			return users.User{}, "", ErrInvalidCredentials
		}
		return users.User{}, "", err
	}

	if !password.Verify(req.Password, u.PasswordHash) {
		return users.User{}, "", ErrInvalidCredentials
	}

	token, err := jwt.Generate(s.jwtSecret, u.ID, tokenExpiry)
	if err != nil {
		return users.User{}, "", err
	}
	return u, token, nil
}

func (s *service) Role(ctx context.Context, userID string) (string, error) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return "", err
	}
	return u.Role, nil
}
