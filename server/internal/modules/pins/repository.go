package pins

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
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

type postgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}
