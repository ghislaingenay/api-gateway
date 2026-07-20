package gateway

import (
	"strings"
	"time"
)

// Route is a single static path-to-service mapping used to resolve the
// downstream upstream for an incoming request.
type Route struct {
	Path                string
	Method              string
	Upstream            string
	AuthRequired        bool
	PermissionsRequired []string
	// CacheTTL overrides the gateway's default response-cache TTL for this
	// route (FEAT-006). Zero means "no override" — the default TTL applies.
	CacheTTL time.Duration
}

// RouteTable resolves the destination Route for an incoming request from a
// static, precedence-ordered list configured at startup. Routes are ordered
// longest-path-first so that a more specific pattern (e.g. "/api/users/*")
// wins over a broader overlapping one (e.g. "/api/*").
type RouteTable struct {
	routes []Route
}

// NewRouteTable builds a RouteTable from the given routes.
func NewRouteTable(routes []Route) *RouteTable {
	ordered := make([]Route, len(routes))
	copy(ordered, routes)
	sortRoutesByPrecedence(ordered)
	return &RouteTable{routes: ordered}
}

// Resolve returns the highest-precedence Route matching method and path.
func (t *RouteTable) Resolve(method, path string) (*Route, bool) {
	for i := range t.routes {
		route := &t.routes[i]
		if !strings.EqualFold(route.Method, method) {
			continue
		}
		if routePathMatches(route.Path, path) {
			return route, true
		}
	}
	return nil, false
}

func sortRoutesByPrecedence(routes []Route) {
	for i := 1; i < len(routes); i++ {
		for j := i; j > 0 && routeWeight(routes[j-1].Path) < routeWeight(routes[j].Path); j-- {
			routes[j-1], routes[j] = routes[j], routes[j-1]
		}
	}
}

// routeWeight ranks a route pattern by specificity: longer patterns (and
// exact, non-wildcard patterns) are more specific and take precedence.
func routeWeight(pattern string) int {
	weight := len(strings.TrimSuffix(pattern, "/*"))
	if !strings.HasSuffix(pattern, "/*") {
		weight++ // exact match outranks a wildcard of the same prefix length
	}
	return weight
}

func routePathMatches(pattern, path string) bool {
	if prefix, ok := strings.CutSuffix(pattern, "/*"); ok {
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	}
	return pattern == path
}
