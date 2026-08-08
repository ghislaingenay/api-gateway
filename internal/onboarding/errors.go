package onboarding

import "errors"

// ErrInvalidOrganizationName means the request's organization_name failed
// validation.
var ErrInvalidOrganizationName = errors.New("invalid organization name")

// ErrTenantLimitReached means the calling user already belongs to the
// configured maximum number of tenants and Onboard refused to create
// another one.
var ErrTenantLimitReached = errors.New("tenant limit reached for user")
