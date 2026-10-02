package collections

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
)

// Collection is a user-created list of pins, with list-level aggregates.
// Private collections are visible only to their owner.
type Collection struct {
	ID          pgtype.UUID `json:"id"`
	UserID      pgtype.UUID `json:"user_id"`
	Name        string      `json:"name"`
	Description *string     `json:"description"`
	IsPrivate   bool        `json:"is_private"`
	CreatedAt   time.Time   `json:"created_at"`
	PinCount    int         `json:"pin_count"`
	CoverURL    string      `json:"cover_url"`
}

// CollectionDetail is a collection with its pins (public, non-hidden only).
type CollectionDetail struct {
	Collection
	Pins []pins.PinListEntry `json:"pins"`
}
