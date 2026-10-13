package pins

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// rowQuerier is the shared subset of *pgxpool.Pool and pgx.Tx that reads a pin's
// photo rows.
type rowQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// pinPhotoListHardCap bounds scanPinPhotos. Validation caps uploads at
// maxPhotosPerPin (5), so the cap can never trigger on legitimate data; it
// only keeps a corrupted photo set from producing an unbounded result.
const pinPhotoListHardCap = 10

// scanPinPhotos reads a pin's photos in position order.
func scanPinPhotos(ctx context.Context, q rowQuerier, pinID string) ([]PinPhoto, error) {
	rows, err := q.Query(ctx, `
		SELECT id, pin_id, photo_url, thumbnail_url, position, created_at
		FROM pin_photos
		WHERE pin_id = $1
		ORDER BY position
		LIMIT $2
	`, pinID, pinPhotoListHardCap)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	photos := []PinPhoto{}
	for rows.Next() {
		var ph PinPhoto
		if err := rows.Scan(&ph.ID, &ph.PinID, &ph.PhotoURL, &ph.ThumbnailURL, &ph.Position, &ph.CreatedAt); err != nil {
			return nil, err
		}
		photos = append(photos, ph)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return photos, nil
}
