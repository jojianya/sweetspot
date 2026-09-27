package auth

import "errors"

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrConflict           = errors.New("email or username already taken")
	// ErrInvalidPassword wraps the underlying reason, so the handler can return
	// the detail to the client while still classifying the failure as a 400.
	ErrInvalidPassword = errors.New("invalid password")
)
