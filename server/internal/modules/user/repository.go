package users

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	Create(ctx context.Context, email, passwordHash, username string) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	GetByUsername(ctx context.Context, username string) (User, error)
	GetByLogin(ctx context.Context, identifier string) (User, error)
	GetByID(ctx context.Context, id string) (User, error)
	CountOwners(ctx context.Context) (int, error)
	UpdateRole(ctx context.Context, id, role string) (User, error)
}

type postgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

func (r *postgresRepository) Create(ctx context.Context, email, passwordHash, username string) (User, error) {
	var u User
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, username)
		VALUES ($1, $2, $3)
		RETURNING id, email, username, avatar_url, socials, role, created_at, updated_at
	`, strings.ToLower(email), passwordHash, username).Scan(
		&u.ID, &u.Email, &u.Username, &u.AvatarURL, &u.Socials, &u.Role, &u.CreatedAt, &u.UpdatedAt,
	)
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

func (r *postgresRepository) CountOwners(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role = 'owner'`).Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (r *postgresRepository) UpdateRole(ctx context.Context, id, role string) (User, error) {
	var u User
	err := r.pool.QueryRow(ctx, `
		UPDATE users SET role = $2 WHERE id = $1
		RETURNING id, email, username, avatar_url, socials, role, created_at, updated_at
	`, id, role).Scan(
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
