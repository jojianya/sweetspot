package collections

import (
	"time"

	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
)

// Collection is a user-created list of pins, with list-level aggregates.
// Private collections are visible only to their owner.
type Collection struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	IsPrivate   bool      `json:"is_private"`
	CreatedAt   time.Time `json:"created_at"`
	PinCount    int       `json:"pin_count"`
	CoverURL    string    `json:"cover_url"`
}

// CollectionDetail is a collection with a page of its pins (public, non-hidden
// only). PinTotal is the number of pins in the whole collection, so a client can
// tell a partial page from a complete one.
type CollectionDetail struct {
	Collection
	Pins     []pins.PinListEntry `json:"pins"`
	PinTotal int                 `json:"pin_total"`
}
