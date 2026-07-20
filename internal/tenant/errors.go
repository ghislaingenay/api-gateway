package tenant

import "errors"

// ErrTenantNotFound means no tenant row exists for the given id.
var ErrTenantNotFound = errors.New("tenant not found")
