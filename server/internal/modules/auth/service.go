package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/pkg/jwt"
	"github.com/jojianya/sweetspot247-backend/pkg/password"
)

const tokenExpiry = 30 * 24 * time.Hour

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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
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
