package pins

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// VisiblePinExists reports whether pinID identifies a pin that exists and is
// visible to other users — i.e. it has not been soft-hidden (deleted or
// moderated away). Modules that let users attach data to a pin (comments,
// favorites, reports, collections) share this one check so the rule
// "hidden pins do not exist" lives in a single place.
func VisiblePinExists(ctx context.Context, pool *pgxpool.Pool, pinID string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pins WHERE id = $1 AND is_hidden = false)`, pinID,
	).Scan(&exists)
	return exists, err
}
