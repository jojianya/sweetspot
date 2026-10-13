package users

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jojianya/sweetspot247-backend/internal/platform/database"
)

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
// result, so we fall back to CountUsers. The id tiebreaker keeps the sort
// stable across pages when two users share a created_at.
func (r *postgresRepository) ListUsers(ctx context.Context, limit, offset int) ([]User, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, email, username, avatar_url, socials, role, created_at, updated_at,
		       COUNT(*) OVER () AS total
		FROM users
		ORDER BY created_at DESC, id DESC
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
	if err := rows.Err(); err != nil {
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

// SearchUsers returns users whose username contains the query
// (case-insensitive) plus the total number of matches, sorted by
// username. The total comes from COUNT(*) OVER () in the same query, so
// the handler can page through matches; if the page is empty we cannot
// read the window function's result and fall back to a count with the
// same predicate. LIKE wildcards in the query are escaped so they match
// literally; the caller has already bounded the query length, limit
// and offset.
func (r *postgresRepository) SearchUsers(ctx context.Context, query string, limit, offset int) ([]User, int, error) {
	escaped := database.EscapeLike(query)
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
	if err := rows.Err(); err != nil {
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
