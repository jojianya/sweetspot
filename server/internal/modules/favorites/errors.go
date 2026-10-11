package favorites

import "errors"

var ErrNotFound = errors.New("pin not found")

// ErrFavoriteNotFound reports an unsave of a pin the caller never saved.
var ErrFavoriteNotFound = errors.New("favorite not found")
