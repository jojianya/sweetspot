package users

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

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
	if errors.As(err, &pgErr) && pgErr.Code == pgCodeUniqueViolation {
		return User{}, ErrUsernameTaken
	}
	if err != nil {
		return User{}, err
	}
	return u, nil
}
