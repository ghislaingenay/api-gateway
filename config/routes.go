package config

import (
	"api-gateway/internal/logger"
	"encoding/json"
	"fmt"
	"os"
)

// BodyFieldEntry is one JSON body field validation rule in a route's
// configured schema (FEAT-007).
type BodyFieldEntry struct {
	Field string `json:"field"`
	// Rule is a go-playground validator tag string, e.g. "required,email".
	Rule string `json:"rule"`
}

// RequiredParamEntry is one required path/query parameter validation rule
// in a route's configured schema (FEAT-007).
type RequiredParamEntry struct {
	Name string `json:"name"`
	// In is "query" or "path".
	In string `json:"in"`
	// Rule is a go-playground validator tag string, e.g. "required,uuid4".
	Rule string `json:"rule"`
}

// RouteEntry is one row of the gateway's static path-to-service
// configuration table.
type RouteEntry struct {
	Path                string   `json:"path"`
	Method              string   `json:"method"`
	Upstream            string   `json:"upstream"`
	AuthRequired        bool     `json:"auth_required"`
	PermissionsRequired []string `json:"permissions_required"`
	// CacheTTLSeconds overrides the gateway's default response-cache TTL for
	// this route (FEAT-006). Zero/omitted means "no override".
	CacheTTLSeconds int `json:"cache_ttl_seconds"`
	// TimeoutSeconds overrides the gateway's default request timeout for
	// this route (FEAT-008). Zero/omitted means "no override".
	TimeoutSeconds int `json:"timeout_seconds"`
	// RetryMaxAttempts overrides the default max retry attempts for GET
	// requests to this route (FEAT-008). Zero/omitted means "no override".
	RetryMaxAttempts int `json:"retry_max_attempts"`
	// BodyRequired rejects an empty body with a 400 when true (FEAT-007).
	BodyRequired bool `json:"body_required"`
	// BodyFields are the JSON body fields validated for this route
	// (FEAT-007). Omitted/empty means the body is not validated.
	BodyFields []BodyFieldEntry `json:"body_fields"`
	// RequiredParams are path/query parameters this route requires to be
	// present and type-valid (FEAT-007). Omitted/empty means none.
	RequiredParams []RequiredParamEntry `json:"required_params"`
}

// Upstream follows $env:ENV_NAME syntax to override the upstream with an environment variable, if present, default to localhost
func overrideUpstreamWithEnv(upstream string) string {
	if len(upstream) > 5 && upstream[:5] == "$env:" {
		envVar := upstream[5:]
		if envValue := os.Getenv(envVar); envValue != "" {
			return envValue
		} else {
			logger.Default().Warn("Missing env for upstream %s", envVar, false)
			return "localhost"
		}
	}
	return upstream
}


// LoadRoutesConfig reads the static route table from the JSON file at
// GATEWAY_ROUTES_FILE (default "config/routes.json").
func LoadRoutesConfig() ([]RouteEntry, error) {
	path := os.Getenv("GATEWAY_ROUTES_FILE")
	if path == "" {
		path = "config/routes.json"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read routes config %q: %w", path, err)
	}

	var routes []RouteEntry
	if err := json.Unmarshal(data, &routes); err != nil {
		return nil, fmt.Errorf("parse routes config %q: %w", path, err)
	}

	for i := range routes {
		route := &routes[i]
		if route.CacheTTLSeconds < 0 {
			return nil, fmt.Errorf("route %s %s: cache_ttl_seconds must not be negative, got %d", route.Method, route.Path, route.CacheTTLSeconds)
		}
		if route.TimeoutSeconds < 0 {
			return nil, fmt.Errorf("route %s %s: timeout_seconds must not be negative, got %d", route.Method, route.Path, route.TimeoutSeconds)
		}
		if route.RetryMaxAttempts < 0 {
			return nil, fmt.Errorf("route %s %s: retry_max_attempts must not be negative, got %d", route.Method, route.Path, route.RetryMaxAttempts)
		}
		for _, p := range route.RequiredParams {
			if p.In != "query" && p.In != "path" {
				return nil, fmt.Errorf("route %s %s: required_params[%q].in must be \"query\" or \"path\", got %q", route.Method, route.Path, p.Name, p.In)
			}
		}

		route.Upstream = overrideUpstreamWithEnv(route.Upstream)
	}

	return routes, nil
}
