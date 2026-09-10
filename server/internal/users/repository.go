package users

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("user not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const userColumns = "id, email, username, avatar_url, socials, role, created_at, updated_at"

func (r *Repository) Create(ctx context.Context, email, passwordHash, username string) (User, error) {
	var u User
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, username)
		VALUES ($1, $2, $3)
		RETURNING `+userColumns+`
	`, strings.ToLower(email), passwordHash, username).Scan(
		&u.ID, &u.Email, &u.Username, &u.AvatarURL, &u.Socials, &u.Role, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return User{}, err
	}
	return u, nil
}

func (r *Repository) GetByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := r.pool.QueryRow(ctx, `
		SELECT `+userColumns+`, password_hash FROM users WHERE email = $1
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

func (r *Repository) GetByUsername(ctx context.Context, username string) (User, error) {
	var u User
	err := r.pool.QueryRow(ctx, `
		SELECT `+userColumns+` FROM users WHERE username = $1
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

func (r *Repository) GetByID(ctx context.Context, id string) (User, error) {
	var u User
	err := r.pool.QueryRow(ctx, `
		SELECT `+userColumns+` FROM users WHERE id = $1
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

func (r *Repository) CountOwners(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role = 'owner'`).Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (r *Repository) UpdateRole(ctx context.Context, id, role string) (User, error) {
	var u User
	err := r.pool.QueryRow(ctx, `
		UPDATE users SET role = $2 WHERE id = $1
		RETURNING `+userColumns+`
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
