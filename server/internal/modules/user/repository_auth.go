package users

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jojianya/sweetspot247-backend/internal/session"
)

func (r *postgresRepository) Create(ctx context.Context, email, passwordHash, username string) (User, error) {
	var u User
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, username)
		VALUES ($1, $2, $3)
		RETURNING id, email, username, avatar_url, socials, role, created_at, updated_at
	`, strings.ToLower(email), passwordHash, username).Scan(
		&u.ID, &u.Email, &u.Username, &u.AvatarURL, &u.Socials, &u.Role, &u.CreatedAt, &u.UpdatedAt,
	)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgCodeUniqueViolation {
		return User{}, ErrDuplicate
	}
	if err != nil {
		return User{}, err
	}
	return u, nil
}

func (r *postgresRepository) GetByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, username, avatar_url, socials, role, created_at, updated_at, password_hash
		FROM users WHERE email = $1
	`, strings.ToLower(email)).Scan(
		&u.ID, &u.Email, &u.Username, &u.AvatarURL, &u.Socials, &u.Role, &u.CreatedAt, &u.UpdatedAt, &u.PasswordHash,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	return u, nil
}

func (r *postgresRepository) GetByUsername(ctx context.Context, username string) (User, error) {
	var u User
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, username, avatar_url, socials, role, created_at, updated_at
		FROM users WHERE username = $1
	`, username).Scan(
		&u.ID, &u.Email, &u.Username, &u.AvatarURL, &u.Socials, &u.Role, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	return u, nil
}

func (r *postgresRepository) GetByLogin(ctx context.Context, identifier string) (User, error) {
	var u User
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, username, avatar_url, socials, role, created_at, updated_at, password_hash
		FROM users
		WHERE email = LOWER($1) OR username = $1
		LIMIT 1
	`, strings.TrimSpace(identifier)).Scan(
		&u.ID, &u.Email, &u.Username, &u.AvatarURL, &u.Socials, &u.Role, &u.CreatedAt, &u.UpdatedAt, &u.PasswordHash,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	return u, nil
}

func (r *postgresRepository) GetByID(ctx context.Context, id string) (User, error) {
	var u User
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, username, avatar_url, socials, role, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(
		&u.ID, &u.Email, &u.Username, &u.AvatarURL, &u.Socials, &u.Role, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	return u, nil
}

func (r *postgresRepository) GetSessionState(ctx context.Context, id string) (session.State, error) {
	var state session.State
	var validAfter *time.Time
	var role string
	err := r.pool.QueryRow(ctx, `
		SELECT role, sessions_valid_after FROM users WHERE id = $1
	`, id).Scan(&role, &validAfter)
	if errors.Is(err, pgx.ErrNoRows) {
		return session.State{}, ErrNotFound
	}
	if err != nil {
		return session.State{}, err
	}
	state.Role = role
	if validAfter != nil {
		state.ValidAfter = *validAfter
	}
	return state, nil
}
