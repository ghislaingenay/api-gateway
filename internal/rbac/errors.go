package rbac

import "errors"

// ErrCacheLoad means roles or permissions could not be loaded from the
// database into the in-memory RoleCache at startup.
var ErrCacheLoad = errors.New("failed to load roles/permissions")
