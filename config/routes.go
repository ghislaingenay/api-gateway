package config

import (
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
	// DeadlineSeconds overrides the gateway's default request deadline for
	// this route (FEAT-008). Zero/omitted means "no override".
	DeadlineSeconds int `json:"deadline_seconds"`
	// RetryMaxAttempts overrides the default max retry attempts for GET
	// requests to this route (FEAT-008). Zero/omitted means "no override".
	RetryMaxAttempts int `json:"retry_max_attempts"`
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

	for _, route := range routes {
		if route.CacheTTLSeconds < 0 {
			return nil, fmt.Errorf("route %s %s: cache_ttl_seconds must not be negative, got %d", route.Method, route.Path, route.CacheTTLSeconds)
		}
		if route.DeadlineSeconds < 0 {
			return nil, fmt.Errorf("route %s %s: deadline_seconds must not be negative, got %d", route.Method, route.Path, route.DeadlineSeconds)
		}
		if route.RetryMaxAttempts < 0 {
			return nil, fmt.Errorf("route %s %s: retry_max_attempts must not be negative, got %d", route.Method, route.Path, route.RetryMaxAttempts)
		}
	}

	return routes, nil
}
