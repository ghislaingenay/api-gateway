package user

import "errors"

// ErrUserNotFound means no user row exists for the given lookup.
var ErrUserNotFound = errors.New("user not found")
