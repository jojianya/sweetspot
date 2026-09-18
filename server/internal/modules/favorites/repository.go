package favorites

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	Save(ctx context.Context, userID, pinID string) error
	Unsave(ctx context.Context, userID, pinID string) error
	IsSaved(ctx context.Context, userID, pinID string) (bool, error)
	PinExists(ctx context.Context, pinID string) (bool, error)
	List(ctx context.Context, userID string) ([]Entry, error)
	ListIDs(ctx context.Context, userID string) ([]string, error)
}

type postgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

func (r *postgresRepository) Save(ctx context.Context, userID, pinID string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO favorites (user_id, pin_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, pin_id) DO NOTHING
	`, userID, pinID)
	return err
}

func (r *postgresRepository) Unsave(ctx context.Context, userID, pinID string) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM favorites WHERE user_id = $1 AND pin_id = $2
	`, userID, pinID)
	return err
}

func (r *postgresRepository) IsSaved(ctx context.Context, userID, pinID string) (bool, error) {
	var saved bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM favorites WHERE user_id = $1 AND pin_id = $2)
	`, userID, pinID).Scan(&saved)
	if err != nil {
		return false, err
	}
	return saved, nil
}

func (r *postgresRepository) PinExists(ctx context.Context, pinID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM pins WHERE id = $1)
	`, pinID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *postgresRepository) List(ctx context.Context, userID string) ([]Entry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.user_id, ST_AsText(p.location) AS location, p.geohash, p.caption, p.category_id, p.is_hidden, p.created_at,
		       COALESCE(pp.thumbnail_url, pp.photo_url, ''), u.username, f.created_at AS saved_at
		FROM favorites f
		JOIN pins p ON p.id = f.pin_id
		LEFT JOIN LATERAL (
			SELECT photo_url, thumbnail_url FROM pin_photos
			WHERE pin_id = p.id
			ORDER BY position
			LIMIT 1
		) pp ON true
		LEFT JOIN users u ON u.id = p.user_id
		WHERE f.user_id = $1 AND p.is_hidden = false
		ORDER BY f.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.Pin.ID, &e.Pin.UserID, &e.Pin.Location, &e.Pin.Geohash, &e.Pin.Caption,
			&e.Pin.CategoryID, &e.Pin.IsHidden, &e.Pin.CreatedAt, &e.CoverURL, &e.Username, &e.SavedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *postgresRepository) ListIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT pin_id::text FROM favorites WHERE user_id = $1 ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}