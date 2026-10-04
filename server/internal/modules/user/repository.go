package users

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
)

type Repository interface {
	Create(ctx context.Context, email, passwordHash, username string) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	GetByUsername(ctx context.Context, username string) (User, error)
	GetByLogin(ctx context.Context, identifier string) (User, error)
	GetByID(ctx context.Context, id string) (User, error)
	// GetSessionState loads the session posture (live role + revocation
	// floor) in one indexed lookup for the auth middleware.
	GetSessionState(ctx context.Context, id string) (middleware.SessionState, error)
	CountUsers(ctx context.Context) (int, error)
	CountOwners(ctx context.Context) (int, error)
	// ListUsers returns a page of users plus the total count. The count comes
	// from COUNT(*) OVER () in the same query (P1.4), with a fallback to the
	// separate CountUsers call when the page is empty and the total is unknown.
	ListUsers(ctx context.Context, limit, offset int) ([]User, int, error)
	// UpdateRole changes a user's role. The last-owner guard is atomic:
	// the owner count and the role write share one transaction that
	// locks every owner row first (SELECT ... FOR UPDATE), so two
	// concurrent demotions serialize — the second re-evaluates the lock
	// after the first commits and can no longer count an owner that was
	// just demoted.
	UpdateRole(ctx context.Context, id, role string) (User, error)
	UpdateProfile(ctx context.Context, id string, patch UpdateProfilePatch) (User, error)
	// SearchUsers returns a page of username matches plus the total number of
	// matches. The total comes from COUNT(*) OVER () in the same query, with a
	// fallback count when the page is empty and the window function has no row
	// to read.
	SearchUsers(ctx context.Context, query string, limit, offset int) ([]User, int, error)
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

func (r *postgresRepository) GetSessionState(ctx context.Context, id string) (middleware.SessionState, error) {
	var state middleware.SessionState
	var validAfter *time.Time
	var role string
	err := r.pool.QueryRow(ctx, `
		SELECT role, sessions_valid_after FROM users WHERE id = $1
	`, id).Scan(&role, &validAfter)
	if errors.Is(err, pgx.ErrNoRows) {
		return middleware.SessionState{}, ErrNotFound
	}
	if err != nil {
		return middleware.SessionState{}, err
	}
	state.Role = role
	if validAfter != nil {
		state.ValidAfter = *validAfter
	}
	return state, nil
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

// ListUsers returns a page of users plus the total count. The count comes
// from COUNT(*) OVER () in the same query (P1.4), avoiding a separate
// round-trip. If the page is empty we cannot read the window function's
// result, so we fall back to CountUsers.
func (r *postgresRepository) ListUsers(ctx context.Context, limit, offset int) ([]User, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, email, username, avatar_url, socials, role, created_at, updated_at,
		       COUNT(*) OVER () AS total
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	users := []User{}
	var total int
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Username, &u.AvatarURL, &u.Socials, &u.Role, &u.CreatedAt, &u.UpdatedAt, &total); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil && err != pgx.ErrNoRows {
		return nil, 0, err
	}

	// Empty page: the window function returns nothing, so we fall back.
	if len(users) == 0 {
		total, err = r.CountUsers(ctx)
		if err != nil {
			return nil, 0, err
		}
	}
	return users, total, nil
}

// UpdateRole changes a user's role. When the change would demote an
// owner, the check and the write happen in one transaction that
// first locks every owner row, so two concurrent demotions cannot
// both pass the count check and leave zero owners.
func (r *postgresRepository) UpdateRole(ctx context.Context, id, role string) (User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)

	// Lock the owner rows before counting. The lock lives in a subquery
	// because Postgres rejects FOR UPDATE next to an aggregate, and
	// ORDER BY id fixes the acquisition order so two concurrent demotions
	// always lock in the same sequence and cannot deadlock. Under READ
	// COMMITTED a concurrent demotion blocks here until this transaction
	// commits; its lock request then re-evaluates against the updated
	// rows, so a just-demoted owner no longer matches role = 'owner'.
	var owners int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM (
			SELECT id FROM users WHERE role = 'owner' ORDER BY id FOR UPDATE
		) AS locked_owners
	`).Scan(&owners); err != nil {
		return User{}, err
	}

	// Re-read the target's role inside the transaction so the check
	// cannot race a concurrent role change on the same row.
	var currentRole string
	if err := tx.QueryRow(ctx, `SELECT role FROM users WHERE id = $1`, id).Scan(&currentRole); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}

	if currentRole == RoleOwner && role != RoleOwner && owners <= 1 {
		return User{}, ErrCannotDemoteLastOwner
	}

	var u User
	if err := tx.QueryRow(ctx, `
		UPDATE users SET role = $2 WHERE id = $1
		RETURNING id, email, username, avatar_url, socials, role, created_at, updated_at
	`, id, role).Scan(
		&u.ID, &u.Email, &u.Username, &u.AvatarURL, &u.Socials, &u.Role, &u.CreatedAt, &u.UpdatedAt,
	); err != nil {
		return User{}, err
	}

	if err := tx.Commit(ctx); err != nil {
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

// SearchUsers returns users whose username contains the query
// (case-insensitive) plus the total number of matches, sorted by
// username. The total comes from COUNT(*) OVER () in the same query, so
// the handler can page through matches; if the page is empty we cannot
// read the window function's result and fall back to a count with the
// same predicate. LIKE wildcards in the query are escaped so they match
// literally; the caller has already bounded the query length, limit
// and offset.
func (r *postgresRepository) SearchUsers(ctx context.Context, query string, limit, offset int) ([]User, int, error) {
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
	rows, err := r.pool.Query(ctx, `
		SELECT id, email, username, avatar_url, socials, role, created_at, updated_at,
		       COUNT(*) OVER () AS total
		FROM users
		WHERE username ILIKE '%' || $1 || '%'
		ORDER BY username
		LIMIT $2 OFFSET $3
	`, escaped, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	users := []User{}
	var total int
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Username, &u.AvatarURL, &u.Socials, &u.Role, &u.CreatedAt, &u.UpdatedAt, &total); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil && err != pgx.ErrNoRows {
		return nil, 0, err
	}

	// Empty page: the window function returns nothing, so we fall back to
	// counting the matches so the caller still learns the true total.
	if len(users) == 0 {
		if err := r.pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM users WHERE username ILIKE '%' || $1 || '%'
		`, escaped).Scan(&total); err != nil {
			return nil, 0, err
		}
	}
	return users, total, nil
}
