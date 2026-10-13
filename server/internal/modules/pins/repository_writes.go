package pins

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (r *postgresRepository) CreatePin(ctx context.Context, pin NewPin) (Pin, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Pin{}, err
	}
	defer tx.Rollback(ctx)

	var p Pin
	err = tx.QueryRow(ctx, `
		INSERT INTO pins (user_id, location, geohash, caption, category_id)
		VALUES ($1, ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography, $4, $5, $6)
		RETURNING id, user_id, ST_AsText(location) AS location, geohash, caption, category_id, is_hidden, views, good_spot_count, created_at
	`, pin.UserID, pin.Lng, pin.Lat, pin.Geohash, pin.Caption, pin.CategoryID).Scan(
		&p.ID, &p.UserID, &p.Location, &p.Geohash, &p.Caption, &p.CategoryID, &p.IsHidden, &p.Views, &p.GoodSpotCount, &p.CreatedAt,
	)
	if err != nil {
		return Pin{}, err
	}

	for i := range pin.PhotoURLs {
		position := int16(i)
		if _, err := tx.Exec(ctx, `
			INSERT INTO pin_photos (pin_id, photo_url, thumbnail_url, position)
			VALUES ($1, $2, $3, $4)
		`, p.ID, pin.PhotoURLs[i], pin.ThumbnailURLs[i], position); err != nil {
			return Pin{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Pin{}, err
	}
	return p, nil
}

func (r *postgresRepository) DeletePin(ctx context.Context, id, userID string, isModerator bool) error {
	// Single mutating path for authorization: the owner-or-moderator check
	// lives in the UPDATE predicate, not in a handler-side load-then-decide,
	// so there is no TOCTOU between an ownership read and the hide. Only
	// visible pins are matched, so deleting a missing or already-hidden pin
	// reports NotFound (consistent with the hidden-pin 404 convention).
	tag, err := r.pool.Exec(ctx, `UPDATE pins SET is_hidden = true WHERE id = $1 AND is_hidden = false AND (user_id = $2 OR $3)`, id, userID, isModerator)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		return nil
	}

	// Classify the miss for a truthful status: missing/already-hidden → 404,
	// visible-but-not-owned (and not a moderator) → 403. This SELECT only
	// decides the error code; the hide itself already happened atomically.
	// ownerID is *string: an orphan pin (author deleted) has a NULL user_id.
	var ownerID *string
	var hidden bool
	err = r.pool.QueryRow(ctx, `SELECT user_id::text, is_hidden FROM pins WHERE id = $1`, id).Scan(&ownerID, &hidden)
	if err != nil {
		return ErrNotFound
	}
	if hidden {
		return ErrNotFound
	}
	return ErrForbidden
}

// UpdatePin applies a patch: caption (nil keeps, empty clears), category_id
// (nil keeps), and optionally a full photo-set replacement. It also returns the
// pin's photo set as it stands after the update, read inside the same
// transaction, so the response can carry the current photos.
//
// Ownership and visibility are decided by the UPDATE predicate rather than by a
// handler-side load-then-decide, matching DeletePin: a missing or already-hidden
// pin reports NotFound (the hidden-pin 404 convention) and a visible pin the
// caller does not own reports Forbidden.
func (r *postgresRepository) UpdatePin(ctx context.Context, id, userID string, isModerator bool, patch UpdatePinPatch) (Pin, []PinPhoto, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Pin{}, nil, err
	}
	defer tx.Rollback(ctx)

	var p Pin
	if err := tx.QueryRow(ctx, `
		UPDATE pins
		SET caption    = COALESCE($4, caption),
		    category_id = COALESCE($5, category_id),
		    updated_at  = now()
		WHERE id = $1
		  AND is_hidden = false
		  AND (user_id = $2 OR $3)
		RETURNING id, user_id, ST_AsText(location) AS location, geohash, caption, category_id, is_hidden, views, created_at
	`, id, userID, isModerator, patch.Caption, patch.CategoryID).Scan(
		&p.ID, &p.UserID, &p.Location, &p.Geohash, &p.Caption, &p.CategoryID, &p.IsHidden, &p.Views, &p.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Pin{}, nil, classifyPinMiss(ctx, tx, id)
		}
		return Pin{}, nil, err
	}

	if patch.Photos != nil {
		if _, err := tx.Exec(ctx, `DELETE FROM pin_photos WHERE pin_id = $1`, id); err != nil {
			return Pin{}, nil, err
		}
		for i, ph := range patch.Photos {
			position := int16(i)
			if _, err := tx.Exec(ctx, `
				INSERT INTO pin_photos (pin_id, photo_url, thumbnail_url, position)
				VALUES ($1, $2, $3, $4)
			`, id, ph.PhotoURL, ph.ThumbnailURL, position); err != nil {
				return Pin{}, nil, err
			}
		}
	}

	photos, err := scanPinPhotos(ctx, tx, id)
	if err != nil {
		return Pin{}, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Pin{}, nil, err
	}
	return p, photos, nil
}

// classifyPinMiss turns a zero-row UPDATE into a truthful error. It runs only
// after the predicate found nothing, and only decides the status code: a pin
// that does not exist or is already hidden is NotFound, a pin that exists and
// is visible but was not matched belongs to someone else.
func classifyPinMiss(ctx context.Context, tx pgx.Tx, id string) error {
	var hidden bool
	if err := tx.QueryRow(ctx, `SELECT is_hidden FROM pins WHERE id = $1`, id).Scan(&hidden); err != nil || hidden {
		return ErrNotFound
	}
	return ErrForbidden
}
