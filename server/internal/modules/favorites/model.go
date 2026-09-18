package favorites

import (
	"time"

	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
)

type Entry struct {
	pins.PinListEntry
	SavedAt time.Time `json:"saved_at"`
}