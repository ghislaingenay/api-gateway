package config

import "errors"

// ErrMissingUpstreamEnv means a route's "$env:ENV_NAME" upstream override
// referenced an environment variable that is unset or empty.
var ErrMissingUpstreamEnv = errors.New("missing environment variable for route upstream override")
