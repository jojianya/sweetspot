package collections

import (
	"context"

	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	"github.com/jojianya/sweetspot247-backend/internal/platform/database"
)

// ListPins returns a page of the collection's visible pins plus the total
// number of them, from COUNT(*) OVER () in the same query. The (position,
// created_at, pin_id) ordering is stable across pages, so offset paging does
// not duplicate or skip a pin even when two rows share a position.
func (r *postgresRepository) ListPins(ctx context.Context, id string, limit, offset int) ([]pins.PinListEntry, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.user_id, ST_AsText(p.location) AS location, p.geohash, p.caption, p.category_id, p.is_hidden, p.views, p.good_spot_count, p.created_at,
		       `+database.CoverPhotoCoalesce+`, u.username,
		       COUNT(*) OVER () AS total
		FROM collection_pins cp
		JOIN pins p ON p.id = cp.pin_id AND p.is_hidden = false
		`+database.CoverPhotoLateral+`
		LEFT JOIN users u ON u.id = p.user_id
		WHERE cp.collection_id = $1
		ORDER BY cp.position, cp.created_at, cp.pin_id
		LIMIT $2 OFFSET $3
	`, id, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	entries := []pins.PinListEntry{}
	var total int
	for rows.Next() {
		var e pins.PinListEntry
		if err := rows.Scan(&e.Pin.ID, &e.Pin.UserID, &e.Pin.Location, &e.Pin.Geohash, &e.Pin.Caption,
			&e.Pin.CategoryID, &e.Pin.IsHidden, &e.Pin.Views, &e.Pin.GoodSpotCount, &e.Pin.CreatedAt, &e.CoverURL, &e.Username, &total); err != nil {
			return nil, 0, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	// Empty page: the window function has no row to read, so count separately.
	if len(entries) == 0 {
		if err := r.pool.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM collection_pins cp
			JOIN pins p ON p.id = cp.pin_id AND p.is_hidden = false
			WHERE cp.collection_id = $1
		`, id).Scan(&total); err != nil {
			return nil, 0, err
		}
	}
	return entries, total, nil
}

func (r *postgresRepository) PinExists(ctx context.Context, pinID string) (bool, error) {
	return pins.VisiblePinExists(ctx, r.pool, pinID)
}

func (r *postgresRepository) AddPin(ctx context.Context, collectionID, pinID string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO collection_pins (collection_id, pin_id, position)
		SELECT $1, $2, COALESCE(MAX(position) + 1, 0)
		FROM collection_pins WHERE collection_id = $1
		ON CONFLICT (collection_id, pin_id) DO NOTHING
	`, collectionID, pinID)
	return err
}

func (r *postgresRepository) RemovePin(ctx context.Context, collectionID, pinID string) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM collection_pins WHERE collection_id = $1 AND pin_id = $2
	`, collectionID, pinID)
	return err
}
