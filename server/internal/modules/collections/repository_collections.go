package collections

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// collectionListHardCap bounds listByUser. The endpoint is not paginated (the
// client shows the full list), so a server-side ceiling keeps a pathological
// account from producing an unbounded result set; the cap is far above any
// real usage.
const collectionListHardCap = 5000

func (r *postgresRepository) UserExists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, id).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *postgresRepository) Create(ctx context.Context, userID, name string, description *string, isPrivate bool) (Collection, error) {
	var c Collection
	c.PinCount = 0
	c.CoverURL = ""
	err := r.pool.QueryRow(ctx, `
		INSERT INTO collections (user_id, name, description, is_private)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, name, description, is_private, created_at
	`, userID, name, description, isPrivate).Scan(&c.ID, &c.UserID, &c.Name, &c.Description, &c.IsPrivate, &c.CreatedAt)
	if err != nil {
		return Collection{}, err
	}
	return c, nil
}

func (r *postgresRepository) Get(ctx context.Context, id string) (Collection, error) {
	var c Collection
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, name, description, is_private, created_at
		FROM collections WHERE id = $1
	`, id).Scan(&c.ID, &c.UserID, &c.Name, &c.Description, &c.IsPrivate, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Collection{}, ErrNotFound
	}
	if err != nil {
		return Collection{}, err
	}
	return c, nil
}

func (r *postgresRepository) ListByUser(ctx context.Context, userID string) ([]Collection, error) {
	return r.listByUser(ctx, userID, false)
}

// ListPublicByUser lists only public collections, for viewers who do not own them.
func (r *postgresRepository) ListPublicByUser(ctx context.Context, userID string) ([]Collection, error) {
	return r.listByUser(ctx, userID, true)
}

func (r *postgresRepository) listByUser(ctx context.Context, userID string, publicOnly bool) ([]Collection, error) {
	visibility := ""
	if publicOnly {
		visibility = "AND c.is_private = false"
	}
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.user_id, c.name, c.description, c.is_private, c.created_at,
		       COUNT(p.id)::int AS pin_count,
		       COALESCE((
		           SELECT COALESCE(pp.thumbnail_url, pp.photo_url, '')
		           FROM collection_pins cp2
		           JOIN pin_photos pp ON pp.pin_id = cp2.pin_id
		           JOIN pins p2 ON p2.id = pp.pin_id AND p2.is_hidden = false
		           WHERE cp2.collection_id = c.id
		           ORDER BY cp2.position, pp.position
		           LIMIT 1
		       ), '') AS cover_url
		FROM collections c
		LEFT JOIN collection_pins cp ON cp.collection_id = c.id
		LEFT JOIN pins p ON p.id = cp.pin_id AND p.is_hidden = false
		WHERE c.user_id = $1 `+visibility+`
		GROUP BY c.id
		ORDER BY c.created_at DESC, c.id DESC
		LIMIT $2
	`, userID, collectionListHardCap)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	collections := []Collection{}
	for rows.Next() {
		var c Collection
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Description, &c.IsPrivate, &c.CreatedAt, &c.PinCount, &c.CoverURL); err != nil {
			return nil, err
		}
		collections = append(collections, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return collections, nil
}

func (r *postgresRepository) Update(ctx context.Context, id, name string, description *string, isPrivate bool, userID string, isModerator bool) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE collections SET name = $2, description = $3, is_private = $4 WHERE id = $1 AND (user_id = $5 OR $6)
	`, id, name, description, isPrivate, userID, isModerator)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM collections WHERE id = $1)`, id).Scan(&exists); err != nil || !exists {
		return ErrNotFound
	}
	return ErrForbidden
}

func (r *postgresRepository) Delete(ctx context.Context, id, userID string, isModerator bool) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM collections WHERE id = $1 AND (user_id = $2 OR $3)`, id, userID, isModerator)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM collections WHERE id = $1)`, id).Scan(&exists); err != nil || !exists {
		return ErrNotFound
	}
	return ErrForbidden
}
