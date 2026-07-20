package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// RouteEntry is one row of the gateway's static path-to-service
// configuration table.
type RouteEntry struct {
	Path                string   `json:"path"`
	Method              string   `json:"method"`
	Upstream            string   `json:"upstream"`
	AuthRequired        bool     `json:"auth_required"`
	PermissionsRequired []string `json:"permissions_required"`
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

	return routes, nil
}
