package collections

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
)

var ErrNotFound = errors.New("collection not found")

type Repository interface {
	UserExists(ctx context.Context, id string) (bool, error)
	Create(ctx context.Context, userID, name string, description *string) (Collection, error)
	Get(ctx context.Context, id string) (Collection, error)
	ListByUser(ctx context.Context, userID string) ([]Collection, error)
	Update(ctx context.Context, id, name string, description *string) error
	Delete(ctx context.Context, id string) error
	ListPins(ctx context.Context, id string) ([]pins.PinListEntry, error)
	PinExists(ctx context.Context, pinID string) (bool, error)
	AddPin(ctx context.Context, collectionID, pinID string) error
	RemovePin(ctx context.Context, collectionID, pinID string) error
}

type postgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

func (r *postgresRepository) UserExists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, id).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *postgresRepository) Create(ctx context.Context, userID, name string, description *string) (Collection, error) {
	var c Collection
	c.PinCount = 0
	c.CoverURL = ""
	err := r.pool.QueryRow(ctx, `
		INSERT INTO collections (user_id, name, description)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, name, description, created_at
	`, userID, name, description).Scan(&c.ID, &c.UserID, &c.Name, &c.Description, &c.CreatedAt)
	if err != nil {
		return Collection{}, err
	}
	return c, nil
}

func (r *postgresRepository) Get(ctx context.Context, id string) (Collection, error) {
	var c Collection
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, name, description, created_at
		FROM collections WHERE id = $1
	`, id).Scan(&c.ID, &c.UserID, &c.Name, &c.Description, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Collection{}, ErrNotFound
	}
	if err != nil {
		return Collection{}, err
	}
	return c, nil
}

func (r *postgresRepository) ListByUser(ctx context.Context, userID string) ([]Collection, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.user_id, c.name, c.description, c.created_at,
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
		WHERE c.user_id = $1
		GROUP BY c.id
		ORDER BY c.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	collections := []Collection{}
	for rows.Next() {
		var c Collection
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Description, &c.CreatedAt, &c.PinCount, &c.CoverURL); err != nil {
			return nil, err
		}
		collections = append(collections, c)
	}
	if err := rows.Err(); err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	return collections, nil
}

func (r *postgresRepository) Update(ctx context.Context, id, name string, description *string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE collections SET name = $2, description = $3 WHERE id = $1
	`, id, name, description)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *postgresRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM collections WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *postgresRepository) ListPins(ctx context.Context, id string) ([]pins.PinListEntry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.user_id, ST_AsText(p.location) AS location, p.geohash, p.caption, p.category_id, p.is_hidden, p.created_at,
		       COALESCE(pp.thumbnail_url, pp.photo_url, ''), u.username
		FROM collection_pins cp
		JOIN pins p ON p.id = cp.pin_id AND p.is_hidden = false
		LEFT JOIN LATERAL (
			SELECT photo_url, thumbnail_url FROM pin_photos
			WHERE pin_id = p.id
			ORDER BY position
			LIMIT 1
		) pp ON true
		LEFT JOIN users u ON u.id = p.user_id
		WHERE cp.collection_id = $1
		ORDER BY cp.position, cp.created_at
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []pins.PinListEntry{}
	for rows.Next() {
		var e pins.PinListEntry
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

func (r *postgresRepository) PinExists(ctx context.Context, pinID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM pins WHERE id = $1 AND is_hidden = false)
	`, pinID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
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
