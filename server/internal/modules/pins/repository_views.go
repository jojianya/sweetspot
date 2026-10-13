package pins

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// RegisterView counts a unique per-account view and returns the current
// count. Anonymous visitors and the pin's owner read the count without
// changing it. Hidden (soft-deleted) pins read as missing, mirroring
// GetPin. The insert and the conditional increment run in one transaction;
// the pin_views primary key serializes concurrent first opens so the count
// moves exactly once per account. Only `views` changes, so the
// pins_updated_at trigger keeps ignoring it.
func (r *postgresRepository) RegisterView(ctx context.Context, id, viewerID string) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var owner string
	var count int64
	err = tx.QueryRow(ctx, `
		SELECT user_id::text, views FROM pins WHERE id = $1 AND is_hidden = false
	`, id).Scan(&owner, &count)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	if viewerID == "" || viewerID == owner {
		if err := tx.Commit(ctx); err != nil {
			return 0, err
		}
		return count, nil
	}

	tag, err := tx.Exec(ctx, `
		INSERT INTO pin_views (pin_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING
	`, id, viewerID)
	if err != nil {
		return 0, err
	}
	if tag.RowsAffected() == 1 {
		if err := tx.QueryRow(ctx, `
			UPDATE pins SET views = views + 1 WHERE id = $1 RETURNING views
		`, id).Scan(&count); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return count, nil
}
