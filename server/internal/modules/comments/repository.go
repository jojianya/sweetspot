package comments

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("comment not found")

type Repository interface {
	ListByPin(ctx context.Context, pinID string) ([]Comment, error)
	Create(ctx context.Context, pinID, userID, body string) (Comment, error)
	Get(ctx context.Context, id string) (Comment, error)
	Hide(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
}

type postgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

func (r *postgresRepository) ListByPin(ctx context.Context, pinID string) ([]Comment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.pin_id, c.user_id, c.body, c.is_hidden, c.created_at, u.username, u.avatar_url
		FROM comments c
		LEFT JOIN users u ON u.id = c.user_id
		WHERE c.pin_id = $1 AND c.is_hidden = false
		ORDER BY c.created_at ASC
	`, pinID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	comments := []Comment{}
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.PinID, &c.UserID, &c.Body, &c.IsHidden, &c.CreatedAt, &c.Username, &c.AvatarURL); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	if err := rows.Err(); err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	return comments, nil
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
