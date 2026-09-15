package pins

import "errors"

var (
	ErrNotFound         = errors.New("pin not found")
	ErrCategoryNotFound = errors.New("category not found")
)
