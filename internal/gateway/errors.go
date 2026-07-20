package gateway

import "errors"

var (
	// ErrNoMatchingRoute means no configured route matched the request's
	// method and path.
	ErrNoMatchingRoute = errors.New("no matching route")
	// ErrTenantInactive means the requesting tenant is disabled or
	// soft-deleted.
	ErrTenantInactive = errors.New("tenant is not active")
)
