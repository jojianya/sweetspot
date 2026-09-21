package social

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
)

var ErrNotFound = errors.New("user not found")

type Repository interface {
	UserExists(ctx context.Context, id string) (bool, error)
	Follow(ctx context.Context, followerID, followeeID string) error
	Unfollow(ctx context.Context, followerID, followeeID string) error
	IsFollowing(ctx context.Context, followerID, followeeID string) (bool, error)
	CountFollowers(ctx context.Context, userID string) (int, error)
	CountFollowing(ctx context.Context, userID string) (int, error)
	CountPins(ctx context.Context, userID string) (int, error)
	Feed(ctx context.Context, userID string, limit int) ([]pins.PinListEntry, error)
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

func (r *postgresRepository) Follow(ctx context.Context, followerID, followeeID string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO follows (follower_id, followee_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, followerID, followeeID)
	return err
}

func (r *postgresRepository) Unfollow(ctx context.Context, followerID, followeeID string) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2
	`, followerID, followeeID)
	return err
}

func (r *postgresRepository) IsFollowing(ctx context.Context, followerID, followeeID string) (bool, error) {
	var following bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id = $1 AND followee_id = $2)
	`, followerID, followeeID).Scan(&following)
	if err != nil {
		return false, err
	}
	return following, nil
}

func (r *postgresRepository) CountFollowers(ctx context.Context, userID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM follows WHERE followee_id = $1`, userID).Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (r *postgresRepository) CountFollowing(ctx context.Context, userID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM follows WHERE follower_id = $1`, userID).Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (r *postgresRepository) CountPins(ctx context.Context, userID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM pins WHERE user_id = $1 AND is_hidden = false
	`, userID).Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// Feed returns the newest public pins from the users that userID follows.
func (r *postgresRepository) Feed(ctx context.Context, userID string, limit int) ([]pins.PinListEntry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.user_id, ST_AsText(p.location) AS location, p.geohash, p.caption, p.category_id, p.is_hidden, p.created_at,
		       COALESCE(pp.thumbnail_url, pp.photo_url, ''), u.username
		FROM follows f
		JOIN pins p ON p.user_id = f.followee_id
		LEFT JOIN LATERAL (
			SELECT photo_url, thumbnail_url FROM pin_photos
			WHERE pin_id = p.id
			ORDER BY position
			LIMIT 1
		) pp ON true
		LEFT JOIN users u ON u.id = p.user_id
		WHERE f.follower_id = $1 AND p.is_hidden = false
		ORDER BY p.created_at DESC
		LIMIT $2
	`, userID, limit)
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
