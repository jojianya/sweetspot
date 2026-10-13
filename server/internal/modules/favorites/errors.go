package favorites

import (
	"errors"

	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
)

// ErrNotFound is pins.ErrNotFound: a missing pin and a hidden pin answer the
// same 404, so the two cases share one sentinel value instead of two
// errors.New with identical text.
var ErrNotFound = pins.ErrNotFound

// ErrFavoriteNotFound reports an unsave of a pin the caller never saved.
var ErrFavoriteNotFound = errors.New("favorite not found")
