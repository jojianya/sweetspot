package pins

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrCategoryNotFound = errors.New("category not found")
var ErrNotFound = errors.New("pin not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) CategoryExists(ctx context.Context, id int) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM categories WHERE id = $1)`, id).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
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

func (r *Repository) CreatePin(ctx context.Context, pin NewPin) (Pin, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Pin{}, err
	}
	defer tx.Rollback(ctx)

	var p Pin
	err = tx.QueryRow(ctx, `
		INSERT INTO pins (user_id, location, geohash, caption, category_id)
		VALUES ($1, ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography, $4, $5, $6)
		RETURNING id, user_id, ST_AsText(location) AS location, geohash, caption, category_id, is_hidden, created_at
	`, pin.UserID, pin.Lng, pin.Lat, pin.Geohash, pin.Caption, pin.CategoryID).Scan(
		&p.ID, &p.UserID, &p.Location, &p.Geohash, &p.Caption, &p.CategoryID, &p.IsHidden, &p.CreatedAt,
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

func (r *Repository) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name FROM categories ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	if err := rows.Err(); err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	return categories, nil
}

type PinDetail struct {
	Pin
	Category  *string    `json:"category"`
	Username  *string    `json:"username"`
	AvatarURL *string    `json:"avatar_url"`
	Photos    []PinPhoto `json:"photos"`
}

func (r *Repository) GetPin(ctx context.Context, id string) (PinDetail, error) {
	var d PinDetail

	err := r.pool.QueryRow(ctx, `
		SELECT p.id, p.user_id, ST_AsText(p.location) AS location, p.geohash, p.caption, p.category_id, p.is_hidden, p.created_at,
		       c.name, u.username, u.avatar_url
		FROM pins p
		LEFT JOIN categories c ON c.id = p.category_id
		LEFT JOIN users u ON u.id = p.user_id
		WHERE p.id = $1
	`, id).Scan(
		&d.ID, &d.UserID, &d.Location, &d.Geohash, &d.Caption, &d.CategoryID, &d.IsHidden, &d.CreatedAt,
		&d.Category, &d.Username, &d.AvatarURL,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return PinDetail{}, ErrNotFound
	}
	if err != nil {
		return PinDetail{}, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, pin_id, photo_url, thumbnail_url, position, created_at
		FROM pin_photos
		WHERE pin_id = $1
		ORDER BY position
	`, id)
	if err != nil {
		return PinDetail{}, err
	}
	defer rows.Close()

	d.Photos = []PinPhoto{}
	for rows.Next() {
		var ph PinPhoto
		if err := rows.Scan(&ph.ID, &ph.PinID, &ph.PhotoURL, &ph.ThumbnailURL, &ph.Position, &ph.CreatedAt); err != nil {
			return PinDetail{}, err
		}
		d.Photos = append(d.Photos, ph)
	}
	if err := rows.Err(); err != nil && err != pgx.ErrNoRows {
		return PinDetail{}, err
	}

	return d, nil
}

type PinListEntry struct {
	Pin
	CoverURL string  `json:"cover_url"`
	Username *string `json:"username"`
}

func (r *Repository) ListPins(ctx context.Context, bbox [4]float64, categoryID *int, limit int) ([]PinListEntry, error) {
	args := []any{bbox[1], bbox[0], bbox[3], bbox[2]}
	if categoryID != nil {
		args = append(args, *categoryID)
	} else {
		args = append(args, nil)
	}
	args = append(args, limit)

	query := `
		SELECT p.id, p.user_id, ST_AsText(p.location) AS location, p.geohash, p.caption, p.category_id, p.is_hidden, p.created_at,
		       COALESCE(pp.thumbnail_url, pp.photo_url, ''), u.username
		FROM pins p
		LEFT JOIN LATERAL (
			SELECT photo_url, thumbnail_url FROM pin_photos
			WHERE pin_id = p.id
			ORDER BY position
			LIMIT 1
		) pp ON true
		LEFT JOIN users u ON u.id = p.user_id
		WHERE p.is_hidden = false
		  AND ($5::int IS NULL OR p.category_id = $5)
		  AND ST_DWithin(p.location, ST_MakeEnvelope($1, $2, $3, $4, 4326)::geography, 0)
		ORDER BY p.created_at DESC
		LIMIT $6
	`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []PinListEntry{}
	for rows.Next() {
		var e PinListEntry
		if err := rows.Scan(&e.Pin.ID, &e.Pin.UserID, &e.Pin.Location, &e.Pin.Geohash, &e.Pin.Caption,
			&e.Pin.CategoryID, &e.Pin.IsHidden, &e.Pin.CreatedAt, &e.CoverURL, &e.Username); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	return entries, nil
}
