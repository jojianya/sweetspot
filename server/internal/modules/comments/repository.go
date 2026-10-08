package comments

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
)

var ErrNotFound = errors.New("comment not found")

// ErrPinNotFound reports that the pin a comment targets does not exist or is
// hidden. The repository translates the comments_pin_id_fkey violation, so
// callers match with errors.Is instead of reading driver error codes.
var ErrPinNotFound = errors.New("pin not found")

// pgCodeForeignKeyViolation is the PostgreSQL SQLSTATE for
// foreign_key_violation. A named constant because pgerrcode is not a
// dependency.
const pgCodeForeignKeyViolation = "23503"

type Repository interface {
	// ListByPin returns a page of comments on the pin plus the total number of
	// visible comments.
	ListByPin(ctx context.Context, pinID string, limit, offset int) ([]Comment, int, error)
	Create(ctx context.Context, pinID, userID, body string) (Comment, error)
	Get(ctx context.Context, id string) (Comment, error)
	Hide(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	// PinExistsVisible reports whether the pin exists and is public; the
	// handler 404s comments on hidden or nonexistent pins.
	PinExistsVisible(ctx context.Context, pinID string) (bool, error)
}

type postgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

func (r *postgresRepository) PinExistsVisible(ctx context.Context, pinID string) (bool, error) {
	return pins.VisiblePinExists(ctx, r.pool, pinID)
}

// ListByPin returns a page of comments on the pin plus the total number of
// visible comments, from COUNT(*) OVER () in the same query. Comments read
// oldest-first, so the ordering column is stable across pages.
func (r *postgresRepository) ListByPin(ctx context.Context, pinID string, limit, offset int) ([]Comment, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.pin_id, c.user_id, c.body, c.is_hidden, c.created_at, u.username, u.avatar_url,
		       COUNT(*) OVER () AS total
		FROM comments c
		LEFT JOIN users u ON u.id = c.user_id
		WHERE c.pin_id = $1 AND c.is_hidden = false
		ORDER BY c.created_at ASC
		LIMIT $2 OFFSET $3
	`, pinID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	comments := []Comment{}
	var total int
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.PinID, &c.UserID, &c.Body, &c.IsHidden, &c.CreatedAt, &c.Username, &c.AvatarURL, &total); err != nil {
			return nil, 0, err
		}
		comments = append(comments, c)
	}
	if err := rows.Err(); err != nil && err != pgx.ErrNoRows {
		return nil, 0, err
	}
	// Empty page: the window function has no row to read, so count separately.
	if len(comments) == 0 {
		if err := r.pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM comments WHERE pin_id = $1 AND is_hidden = false
		`, pinID).Scan(&total); err != nil {
			return nil, 0, err
		}
	}
	return comments, total, nil
}

func (r *postgresRepository) Create(ctx context.Context, pinID, userID, body string) (Comment, error) {
	var c Comment
	err := r.pool.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO comments (pin_id, user_id, body)
			VALUES ($1, $2, $3)
			RETURNING id, pin_id, user_id, body, is_hidden, created_at
		)
		SELECT i.id, i.pin_id, i.user_id, i.body, i.is_hidden, i.created_at, u.username, u.avatar_url
		FROM inserted i
		LEFT JOIN users u ON u.id = i.user_id
	`, pinID, userID, body).Scan(
		&c.ID, &c.PinID, &c.UserID, &c.Body, &c.IsHidden, &c.CreatedAt, &c.Username, &c.AvatarURL,
	)
	if err != nil {
		// Safety net: the pin vanished between the visibility check and the
		// insert (or raced a hide) — report it as a missing pin, not a 500.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgCodeForeignKeyViolation {
			return Comment{}, ErrPinNotFound
		}
		return Comment{}, err
	}
	return c, nil
}

func (r *postgresRepository) Get(ctx context.Context, id string) (Comment, error) {
	var c Comment
	err := r.pool.QueryRow(ctx, `
		SELECT id, pin_id, user_id, body, is_hidden, created_at
		FROM comments WHERE id = $1
	`, id).Scan(&c.ID, &c.PinID, &c.UserID, &c.Body, &c.IsHidden, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Comment{}, ErrNotFound
	}
	if err != nil {
		return Comment{}, err
	}
	return c, nil
}

func (r *postgresRepository) Hide(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE comments SET is_hidden = true WHERE id = $1`, id)
	return err
}

func (r *postgresRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM comments WHERE id = $1`, id)
	return err
}
