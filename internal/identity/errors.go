package identity

import "errors"

// ErrNotMember means the authenticated caller has no tenant_users row for
// the requested tenant.
var ErrNotMember = errors.New("caller is not a member of the requested tenant")
