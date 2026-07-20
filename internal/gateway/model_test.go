package gateway

import "testing"

func TestRouteTable_Resolve(t *testing.T) {
	routes := []Route{
		{Path: "/api/orders/*", Method: "GET", Upstream: "http://wildcard-orders"},
		{Path: "/api/orders/special", Method: "GET", Upstream: "http://exact-orders-special"},
		{Path: "/api/orders", Method: "POST", Upstream: "http://create-orders"},
		{Path: "/api/*", Method: "GET", Upstream: "http://catch-all"},
	}
	table := NewRouteTable(routes)

	tests := []struct {
		name         string
		method       string
		path         string
		wantMatch    bool
		wantUpstream string
	}{
		{"exact path beats overlapping wildcard", "GET", "/api/orders/special", true, "http://exact-orders-special"},
		{"more specific wildcard beats broader one", "GET", "/api/orders/123", true, "http://wildcard-orders"},
		{"wildcard matches its own root", "GET", "/api/orders", true, "http://wildcard-orders"},
		{"broadest wildcard used when nothing more specific matches", "GET", "/api/users/1", true, "http://catch-all"},
		{"method mismatch does not match", "DELETE", "/api/orders/123", false, ""},
		{"unmatched path returns no route", "GET", "/unknown", false, ""},
		{"method is case-insensitive", "get", "/api/orders/123", true, "http://wildcard-orders"},
		{"exact non-wildcard route matches its method only", "POST", "/api/orders", true, "http://create-orders"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			route, ok := table.Resolve(tt.method, tt.path)
			if ok != tt.wantMatch {
				t.Fatalf("Resolve() ok = %v, want %v", ok, tt.wantMatch)
			}
			if !tt.wantMatch {
				return
			}
			if route.Upstream != tt.wantUpstream {
				t.Errorf("Resolve() upstream = %q, want %q", route.Upstream, tt.wantUpstream)
			}
		})
	}
}
