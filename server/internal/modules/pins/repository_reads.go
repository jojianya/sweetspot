package pins

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (r *postgresRepository) CategoryExists(ctx context.Context, id int) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM categories WHERE id = $1)`, id).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *postgresRepository) GetPin(ctx context.Context, id string) (PinDetail, error) {
	var d PinDetail

	err := r.pool.QueryRow(ctx, `
		SELECT p.id, p.user_id, ST_AsText(p.location) AS location, p.geohash, p.caption, p.category_id, p.is_hidden, p.views, p.good_spot_count, p.created_at,
		       c.name, u.username, u.avatar_url
		FROM pins p
		LEFT JOIN categories c ON c.id = p.category_id
		LEFT JOIN users u ON u.id = p.user_id
		WHERE p.id = $1
	`, id).Scan(
		&d.ID, &d.UserID, &d.Location, &d.Geohash, &d.Caption, &d.CategoryID, &d.IsHidden, &d.Views, &d.GoodSpotCount, &d.CreatedAt,
		&d.Category, &d.Username, &d.AvatarURL,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return PinDetail{}, ErrNotFound
	}
	if err != nil {
		return PinDetail{}, err
	}

	d.Photos, err = scanPinPhotos(ctx, r.pool, id)
	if err != nil {
		return PinDetail{}, err
	}

	return d, nil
}

// PinVisible reports whether the pin exists and is not soft-hidden, delegating
// to the shared VisiblePinExists helper.
func (r *postgresRepository) PinVisible(ctx context.Context, id string) (bool, error) {
	return VisiblePinExists(ctx, r.pool, id)
}

func (r *postgresRepository) UserExists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, id).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}
