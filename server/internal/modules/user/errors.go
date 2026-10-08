package users

import "errors"

var (
	ErrNotFound              = errors.New("user not found")
	ErrCannotChangeOwnRole   = errors.New("cannot change your own role")
	ErrCannotDemoteLastOwner = errors.New("cannot demote the last owner")
	ErrUsernameTaken         = errors.New("username already taken")
	// ErrDuplicate reports a unique-constraint violation on user creation:
	// the email or the username is already registered. The repository
	// translates the driver error; callers match with errors.Is.
	ErrDuplicate = errors.New("email or username already taken")
)
