package users

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	Create(ctx context.Context, email, passwordHash, username string) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	GetByUsername(ctx context.Context, username string) (User, error)
	GetByLogin(ctx context.Context, identifier string) (User, error)
	GetByID(ctx context.Context, id string) (User, error)
	CountUsers(ctx context.Context) (int, error)
	CountOwners(ctx context.Context) (int, error)
	ListUsers(ctx context.Context, limit, offset int) ([]User, error)
	UpdateRole(ctx context.Context, id, role string) (User, error)
	UpdateProfile(ctx context.Context, id string, patch UpdateProfilePatch) (User, error)
	SearchUsers(ctx context.Context, query string, limit int) ([]User, error)
}

// UpdateProfilePatch carries the fields to change on the caller's own profile.
// A nil field means "leave unchanged"; AvatarURL and Socials are only ever set
// (never cleared) through this endpoint.
type UpdateProfilePatch struct {
	Username  *string
	AvatarURL *string
	Socials   *map[string]any
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

func (r *postgresRepository) CountUsers(ctx context.Context) (int, error) {
	var n int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// ListUsers returns all users (newest first) for the owner's role-management
// console. Only public fields are scanned — never password_hash.
func (r *postgresRepository) ListUsers(ctx context.Context, limit, offset int) ([]User, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, email, username, avatar_url, socials, role, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Username, &u.AvatarURL, &u.Socials, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	return users, nil
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

// UpdateProfile applies a partial update to the user's own profile. A nil patch
// field keeps the current value (COALESCE over a NULL parameter). A duplicate
// username surfaces as ErrUsernameTaken.
func (r *postgresRepository) UpdateProfile(ctx context.Context, id string, patch UpdateProfilePatch) (User, error) {
	var socials any
	if patch.Socials != nil {
		socials = *patch.Socials
	}

	var u User
	err := r.pool.QueryRow(ctx, `
		UPDATE users SET
			username   = COALESCE($2, username),
			avatar_url = COALESCE($3, avatar_url),
			socials    = COALESCE($4::jsonb, socials)
		WHERE id = $1
		RETURNING id, email, username, avatar_url, socials, role, created_at, updated_at
	`, id, patch.Username, patch.AvatarURL, socials).Scan(
		&u.ID, &u.Email, &u.Username, &u.AvatarURL, &u.Socials, &u.Role, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return User{}, ErrUsernameTaken
	}
	if err != nil {
		return User{}, err
	}
	return u, nil
}

// SearchUsers returns users whose username contains the query (case-insensitive),
// sorted by username. LIKE wildcards in the query are escaped so they match
// literally; the caller has already bounded the query length and limit.
func (r *postgresRepository) SearchUsers(ctx context.Context, query string, limit int) ([]User, error) {
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
	rows, err := r.pool.Query(ctx, `
		SELECT id, email, username, avatar_url, socials, role, created_at, updated_at
		FROM users
		WHERE username ILIKE '%' || $1 || '%'
		ORDER BY username
		LIMIT $2
	`, escaped, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Username, &u.AvatarURL, &u.Socials, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	return users, nil
}
