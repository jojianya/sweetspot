package pins

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/platform/database"
)

type Repository interface {
	CategoryExists(ctx context.Context, id int) (bool, error)
	CreatePin(ctx context.Context, pin NewPin) (Pin, error)
	DeletePin(ctx context.Context, id, userID string, isModerator bool) error
	ListCategories(ctx context.Context) ([]Category, error)
	GetPin(ctx context.Context, id string) (PinDetail, error)
	ListPins(ctx context.Context, bbox [4]float64, categoryID *int, limit int) ([]PinListEntry, error)
	ListTrending(ctx context.Context, bbox [4]float64, limit int) ([]TrendingPin, error)
	ListByUser(ctx context.Context, userID string, limit int) ([]PinListEntry, error)
	SearchPins(ctx context.Context, query string, limit int) ([]PinListEntry, error)
	// UpdatePin applies the patch and returns the pin plus its photo set after
	// the change, so the handler can answer with the pin's current photos
	// instead of leaving the client to refetch them. Authorization lives in
	// the UPDATE predicate (owner or moderator, visible pins only), so there is
	// no TOCTOU between an ownership read and the write.
	UpdatePin(ctx context.Context, id, userID string, isModerator bool, patch UpdatePinPatch) (Pin, []PinPhoto, error)
	// PinVisible reports whether the pin exists and is not soft-hidden. It
	// delegates to the shared VisiblePinExists helper so the "hidden pins do
	// not exist" rule stays in one place.
	PinVisible(ctx context.Context, id string) (bool, error)
	RegisterView(ctx context.Context, id, viewerID string) (int64, error)
	UserExists(ctx context.Context, id string) (bool, error)
}

type NewPin struct {
	UserID        string
	Lat           float64
	Lng           float64
	Caption       *string
	CategoryID    int
	PhotoURLs     []string
	ThumbnailURLs []string
	Geohash       string
}

// NewPhoto is a re-encoded image ready to persist.
type NewPhoto struct {
	PhotoURL     string
	ThumbnailURL string
}

// UpdatePinPatch carries editable pin fields. A nil Photos keeps the current
// photo set; a non-nil (even empty is rejected by the handler) one replaces it.
type UpdatePinPatch struct {
	Caption    *string
	CategoryID *int
	Photos     []NewPhoto
}

type postgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

func (r *postgresRepository) CategoryExists(ctx context.Context, id int) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM categories WHERE id = $1)`, id).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

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
		RETURNING id, user_id, ST_AsText(location) AS location, geohash, caption, category_id, is_hidden, views, created_at
	`, pin.UserID, pin.Lng, pin.Lat, pin.Geohash, pin.Caption, pin.CategoryID).Scan(
		&p.ID, &p.UserID, &p.Location, &p.Geohash, &p.Caption, &p.CategoryID, &p.IsHidden, &p.Views, &p.CreatedAt,
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
	var ownerID string
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

func (r *postgresRepository) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, slug FROM categories ORDER BY id`)
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
	if err := rows.Err(); err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	return categories, nil
}

func (r *postgresRepository) GetPin(ctx context.Context, id string) (PinDetail, error) {
	var d PinDetail

	err := r.pool.QueryRow(ctx, `
		SELECT p.id, p.user_id, ST_AsText(p.location) AS location, p.geohash, p.caption, p.category_id, p.is_hidden, p.views, p.created_at,
		       c.name, u.username, u.avatar_url
		FROM pins p
		LEFT JOIN categories c ON c.id = p.category_id
		LEFT JOIN users u ON u.id = p.user_id
		WHERE p.id = $1
	`, id).Scan(
		&d.ID, &d.UserID, &d.Location, &d.Geohash, &d.Caption, &d.CategoryID, &d.IsHidden, &d.Views, &d.CreatedAt,
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

// rowQuerier is the shared subset of *pgxpool.Pool and pgx.Tx that reads a pin's
// photo rows.
type rowQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// scanPinPhotos reads a pin's photos in position order.
func scanPinPhotos(ctx context.Context, q rowQuerier, pinID string) ([]PinPhoto, error) {
	rows, err := q.Query(ctx, `
		SELECT id, pin_id, photo_url, thumbnail_url, position, created_at
		FROM pin_photos
		WHERE pin_id = $1
		ORDER BY position
	`, pinID)
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
	if err := rows.Err(); err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	return photos, nil
}

// pinListEntrySelect is the shared SELECT shape for list/search results: the
// pin columns plus the first photo's cover URL and the author's username.
const pinListEntrySelect = `
	SELECT p.id, p.user_id, ST_AsText(p.location) AS location, p.geohash, p.caption, p.category_id, p.is_hidden, p.views, p.created_at,
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
		ORDER BY p.created_at DESC
		LIMIT $6`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanPinListEntries(rows)
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
	SELECT p.id, p.user_id, ST_AsText(p.location) AS location, p.geohash, p.caption, p.category_id, p.is_hidden, p.views, p.created_at,
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
		ORDER BY score DESC, p.created_at DESC
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
			&e.Pin.CategoryID, &e.Pin.IsHidden, &e.Pin.Views, &e.Pin.CreatedAt, &e.CoverURL, &e.Username,
			&e.CommentCount, &e.Score); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	return entries, nil
}

func (r *postgresRepository) SearchPins(ctx context.Context, query string, limit int) ([]PinListEntry, error) {
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)

	querySQL := pinListEntrySelect + `
		  AND (p.caption ILIKE '%' || $1 || '%' OR u.username ILIKE '%' || $1 || '%')
		ORDER BY p.created_at DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, querySQL, escaped, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanPinListEntries(rows)
}

func (r *postgresRepository) ListByUser(ctx context.Context, userID string, limit int) ([]PinListEntry, error) {
	query := pinListEntrySelect + `
		  AND p.user_id = $1
		ORDER BY p.created_at DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanPinListEntries(rows)
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

// PinVisible reports whether the pin exists and is not soft-hidden, delegating
// to the shared VisiblePinExists helper.
func (r *postgresRepository) PinVisible(ctx context.Context, id string) (bool, error) {
	return VisiblePinExists(ctx, r.pool, id)
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

// scanPinListEntries maps the shared list/search result rows into entries.
func scanPinListEntries(rows pgx.Rows) ([]PinListEntry, error) {
	entries := []PinListEntry{}
	for rows.Next() {
		var e PinListEntry
		if err := rows.Scan(&e.Pin.ID, &e.Pin.UserID, &e.Pin.Location, &e.Pin.Geohash, &e.Pin.Caption,
			&e.Pin.CategoryID, &e.Pin.IsHidden, &e.Pin.Views, &e.Pin.CreatedAt, &e.CoverURL, &e.Username); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	return entries, nil
}

func (r *postgresRepository) UserExists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, id).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

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
