package reactions

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
)

// Repository is the reactions data layer. It deliberately does not know the
// own-pin or visibility rules; the service decides and the repository answers.
type Repository interface {
	// React records a reaction, ignoring a duplicate. The primary key
	// (pin_id, user_id) makes the insert idempotent, so two taps from one user
	// leave one row and the trigger adds one.
	React(ctx context.Context, userID, pinID string) error
	// Unreact removes the caller's reaction. Deleting zero rows is not an
	// error: the caller is then simply not reacted, which is a valid end state
	// and must not turn an undo race into a failure.
	Unreact(ctx context.Context, userID, pinID string) error
	// ReactedByMe reports whether the caller already reacted.
	ReactedByMe(ctx context.Context, userID, pinID string) (bool, error)
	// Count reads the pin's authoritative reaction count, maintained by the
	// 0023 trigger.
	Count(ctx context.Context, pinID string) (int, error)
	// PinExistsVisible reports whether the pin exists and is not hidden.
	PinExistsVisible(ctx context.Context, pinID string) (bool, error)
	// PinOwner returns the pin's author, or nil for an orphan pin whose author
	// was deleted (pins.user_id is ON DELETE SET NULL). A nil owner can never
	// match the caller, so it is not the caller's own pin.
	PinOwner(ctx context.Context, pinID string) (*string, error)
}

type postgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

func (r *postgresRepository) React(ctx context.Context, userID, pinID string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO good_spots (pin_id, user_id)
		VALUES ($2, $1)
		ON CONFLICT (pin_id, user_id) DO NOTHING
	`, userID, pinID)
	return err
}

func (r *postgresRepository) Unreact(ctx context.Context, userID, pinID string) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM good_spots WHERE pin_id = $1 AND user_id = $2
	`, pinID, userID)
	return err
}

func (r *postgresRepository) ReactedByMe(ctx context.Context, userID, pinID string) (bool, error) {
	// EXISTS on the primary key (pin_id, user_id) is an index probe, not a scan.
	var reacted bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM good_spots WHERE pin_id = $1 AND user_id = $2)
	`, pinID, userID).Scan(&reacted)
	if err != nil {
		return false, err
	}
	return reacted, nil
}

func (r *postgresRepository) Count(ctx context.Context, pinID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT good_spot_count FROM pins WHERE id = $1
	`, pinID).Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (r *postgresRepository) PinExistsVisible(ctx context.Context, pinID string) (bool, error) {
	// Shared with comments, favorites, reports and collections so the rule
	// "hidden pins do not exist" lives in exactly one place.
	return pins.VisiblePinExists(ctx, r.pool, pinID)
}

func (r *postgresRepository) PinOwner(ctx context.Context, pinID string) (*string, error) {
	var owner *string
	err := r.pool.QueryRow(ctx, `
		SELECT user_id FROM pins WHERE id = $1
	`, pinID).Scan(&owner)
	if err != nil {
		return nil, err
	}
	return owner, nil
}
