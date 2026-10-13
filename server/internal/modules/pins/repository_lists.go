package pins

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jojianya/sweetspot247-backend/internal/platform/database"
)

// categoryListHardCap bounds ListCategories. The table is an 8-row seed, so
// the cap can never trigger on legitimate data; it only guards the endpoint
// against returning an unbounded set if the seed ever grows.
const categoryListHardCap = 100

func (r *postgresRepository) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, slug FROM categories ORDER BY id LIMIT $1`, categoryListHardCap)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return categories, nil
}

// pinListEntrySelect is the shared SELECT shape for list/search results: the
// pin columns plus the first photo's cover URL and the author's username.
const pinListEntrySelect = `
	SELECT p.id, p.user_id, ST_AsText(p.location) AS location, p.geohash, p.caption, p.category_id, p.is_hidden, p.views, p.good_spot_count, p.created_at,
	       ` + database.CoverPhotoCoalesce + `, u.username
	FROM pins p
	` + database.CoverPhotoLateral + `
	LEFT JOIN users u ON u.id = p.user_id
	WHERE p.is_hidden = false`

func (r *postgresRepository) ListPins(ctx context.Context, bbox [4]float64, categoryID *int, limit int) ([]PinListEntry, error) {
	// bbox is [minLat, minLng, maxLat, maxLng]; ST_MakeEnvelope(xmin, ymin, xmax, ymax)
	// expects (minLng, minLat, maxLng, maxLat).
	args := []any{bbox[1], bbox[0], bbox[3], bbox[2]}
	if categoryID != nil {
		args = append(args, *categoryID)
	} else {
		args = append(args, nil)
	}
	args = append(args, limit)

	query := pinListEntrySelect + `
		  AND ($5::int IS NULL OR p.category_id = $5)
		  AND ST_Intersects(p.location, ST_MakeEnvelope($1, $2, $3, $4, 4326))
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT $6`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return ScanPinListEntries(rows)
}

// trendingPinSelect is trending's SELECT: the shared list-entry shape plus the
// comment count and a hotness score that decays with age (in hours). The score
// keeps recent pins competitive while still rewarding engagement:
//
//	score = (views + 5*comments) / (age_hours + 2)
//
// The comment count uses a LATERAL join per pin so it can use the
// comments_pin_idx (pin_id, created_at) instead of scanning the whole
// comments table. This is P1.1.
const trendingPinSelect = `
	SELECT p.id, p.user_id, ST_AsText(p.location) AS location, p.geohash, p.caption, p.category_id, p.is_hidden, p.views, p.good_spot_count, p.created_at,
	       COALESCE(pp.thumbnail_url, pp.photo_url, ''), u.username,
	       COALESCE(c.comment_count, 0)::int AS comment_count,
	       ROUND(((p.views + 5.0 * COALESCE(c.comment_count, 0)) /
	              (EXTRACT(EPOCH FROM (now() - p.created_at)) / 3600.0 + 2.0))::numeric, 2) AS score
	FROM pins p
	LEFT JOIN LATERAL (
		SELECT photo_url, thumbnail_url FROM pin_photos
		WHERE pin_id = p.id
		ORDER BY position
		LIMIT 1
	) pp ON true
	LEFT JOIN users u ON u.id = p.user_id
	LEFT JOIN LATERAL (
		SELECT COUNT(*) AS comment_count
		FROM comments
		WHERE pin_id = p.id AND is_hidden = false
	) c ON true
	WHERE p.is_hidden = false`

func (r *postgresRepository) ListTrending(ctx context.Context, bbox [4]float64, limit int) ([]TrendingPin, error) {
	// bbox is [minLat, minLng, maxLat, maxLng]; ST_MakeEnvelope(xmin, ymin, xmax, ymax)
	// expects (minLng, minLat, maxLng, maxLat).
	query := trendingPinSelect + `
		  AND ST_Intersects(p.location, ST_MakeEnvelope($1, $2, $3, $4, 4326))
		ORDER BY score DESC, p.created_at DESC, p.id DESC
		LIMIT $5`

	rows, err := r.pool.Query(ctx, query, bbox[1], bbox[0], bbox[3], bbox[2], limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTrendingPins(rows)
}

// scanTrendingPins maps trending rows (list-entry columns + comment_count,
// score) into entries.
func scanTrendingPins(rows pgx.Rows) ([]TrendingPin, error) {
	entries := []TrendingPin{}
	for rows.Next() {
		var e TrendingPin
		if err := rows.Scan(&e.Pin.ID, &e.Pin.UserID, &e.Pin.Location, &e.Pin.Geohash, &e.Pin.Caption,
			&e.Pin.CategoryID, &e.Pin.IsHidden, &e.Pin.Views, &e.Pin.GoodSpotCount, &e.Pin.CreatedAt, &e.CoverURL, &e.Username,
			&e.CommentCount, &e.Score); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *postgresRepository) SearchPins(ctx context.Context, query string, limit int) ([]PinListEntry, error) {
	escaped := database.EscapeLike(query)

	querySQL := pinListEntrySelect + `
		  AND (p.caption ILIKE '%' || $1 || '%' OR u.username ILIKE '%' || $1 || '%')
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, querySQL, escaped, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return ScanPinListEntries(rows)
}

func (r *postgresRepository) ListByUser(ctx context.Context, userID string, limit int) ([]PinListEntry, error) {
	query := pinListEntrySelect + `
		  AND p.user_id = $1
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return ScanPinListEntries(rows)
}

// ScanPinListEntries maps the shared list/search result rows into entries.
// The row must carry the pinListEntrySelect columns in order; collections,
// favorites and social reuse this so the scan shape lives in exactly one
// place.
func ScanPinListEntries(rows pgx.Rows) ([]PinListEntry, error) {
	entries := []PinListEntry{}
	for rows.Next() {
		var e PinListEntry
		if err := rows.Scan(&e.Pin.ID, &e.Pin.UserID, &e.Pin.Location, &e.Pin.Geohash, &e.Pin.Caption,
			&e.Pin.CategoryID, &e.Pin.IsHidden, &e.Pin.Views, &e.Pin.GoodSpotCount, &e.Pin.CreatedAt, &e.CoverURL, &e.Username); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}
