package collections

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
)

var (
	ErrNotFound     = errors.New("collection not found")
	ErrForbidden    = errors.New("forbidden")
	ErrPinNotFound  = pins.ErrNotFound
	ErrUserNotFound = users.ErrNotFound
)

type Repository interface {
	UserExists(ctx context.Context, id string) (bool, error)
	Create(ctx context.Context, userID, name string, description *string, isPrivate bool) (Collection, error)
	Get(ctx context.Context, id string) (Collection, error)
	ListByUser(ctx context.Context, userID string) ([]Collection, error)
	// ListPublicByUser omits private collections for viewers who do not own them.
	ListPublicByUser(ctx context.Context, userID string) ([]Collection, error)
	Update(ctx context.Context, id, name string, description *string, isPrivate bool, userID string, isModerator bool) error
	Delete(ctx context.Context, id, userID string, isModerator bool) error
	// ListPins returns a page of the collection's visible pins plus the total
	// number of them.
	ListPins(ctx context.Context, id string, limit, offset int) ([]pins.PinListEntry, int, error)
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
