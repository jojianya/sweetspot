package users

import "errors"

var (
	ErrNotFound              = errors.New("user not found")
	ErrCannotChangeOwnRole   = errors.New("cannot change your own role")
	ErrCannotDemoteLastOwner = errors.New("cannot demote the last owner")
	ErrUsernameTaken         = errors.New("username already taken")
)
