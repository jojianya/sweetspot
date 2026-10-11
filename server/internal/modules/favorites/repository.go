package favorites

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	"github.com/jojianya/sweetspot247-backend/internal/platform/database"
)

type Repository interface {
	Save(ctx context.Context, userID, pinID string) error
	Unsave(ctx context.Context, userID, pinID string) error
	IsSaved(ctx context.Context, userID, pinID string) (bool, error)
	PinExists(ctx context.Context, pinID string) (bool, error)
	// List returns a page of saved pins plus the total number of matches.
	List(ctx context.Context, userID string, limit, offset int) ([]Entry, int, error)
	// ListIDs returns every saved pin id for the user and is deliberately not
	// paginated: the client answers "is this pin saved?" by testing membership,
	// so a page would report older pins as unsaved. It carries a hard cap
	// instead — see listIDsHardCap.
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
	return pins.VisiblePinExists(ctx, r.pool, pinID)
}

// List returns a page of saved pins plus the total number of saved pins for
// the user, from COUNT(*) OVER () in the same query.
func (r *postgresRepository) List(ctx context.Context, userID string, limit, offset int) ([]Entry, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.user_id, ST_AsText(p.location) AS location, p.geohash, p.caption, p.category_id, p.is_hidden, p.views, p.good_spot_count, p.created_at,
		       `+database.CoverPhotoCoalesce+`, u.username, f.created_at AS saved_at,
		       COUNT(*) OVER () AS total
		FROM favorites f
		JOIN pins p ON p.id = f.pin_id
		`+database.CoverPhotoLateral+`
		LEFT JOIN users u ON u.id = p.user_id
		WHERE f.user_id = $1 AND p.is_hidden = false
		ORDER BY f.created_at DESC, f.pin_id DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	entries := []Entry{}
	var total int
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.Pin.ID, &e.Pin.UserID, &e.Pin.Location, &e.Pin.Geohash, &e.Pin.Caption,
			&e.Pin.CategoryID, &e.Pin.IsHidden, &e.Pin.Views, &e.Pin.GoodSpotCount, &e.Pin.CreatedAt, &e.CoverURL, &e.Username, &e.SavedAt, &total); err != nil {
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
			SELECT COUNT(*) FROM favorites f
			JOIN pins p ON p.id = f.pin_id
			WHERE f.user_id = $1 AND p.is_hidden = false
		`, userID).Scan(&total); err != nil {
			return nil, 0, err
		}
	}
	return entries, total, nil
}

// listIDsHardCap bounds ListIDs. The client tests membership to answer "is this
// pin saved?", so the list cannot be paginated without reporting older pins as
// unsaved; a cap keeps a pathological account from returning an unbounded
// result set, and one id per row makes even the cap cheap.
const listIDsHardCap = 5000

// ListIDs returns every saved pin id for the user, newest first, capped at
// listIDsHardCap.
func (r *postgresRepository) ListIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT f.pin_id::text FROM favorites f
		JOIN pins p ON p.id = f.pin_id AND p.is_hidden = false
		WHERE f.user_id = $1 ORDER BY f.created_at DESC, f.pin_id DESC
		LIMIT $2
	`, userID, listIDsHardCap)
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
