package profile

import "errors"

// ErrProfileNotFound means no profile row exists for the given lookup.
var ErrProfileNotFound = errors.New("profile not found")
