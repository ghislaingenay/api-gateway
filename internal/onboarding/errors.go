package onboarding

import "errors"

// ErrInvalidOrganizationName means the request's organization_name failed
// validation.
var ErrInvalidOrganizationName = errors.New("invalid organization name")
