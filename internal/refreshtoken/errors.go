package refreshtoken

import "errors"

// ErrNotFound means no refresh token row exists for the given hash.
var ErrNotFound = errors.New("refresh token not found")
