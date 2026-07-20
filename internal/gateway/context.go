package gateway

import "context"

// routeContextKey is unexported so only this package's WithRoute/
// RouteFromContext pair can set or read the value, mirroring
// auth.WithClaims/ClaimsFromContext.
type routeContextKey struct{}

// WithRoute returns a context carrying an already-resolved Route, so a
// middleware that resolves the route for its own purposes (e.g.
// cache.CacheMiddleware reading CacheTTL) can share that resolution with
// NewHandler instead of both re-running RouteTable.Resolve for the same
// request.
func WithRoute(ctx context.Context, route *Route) context.Context {
	return context.WithValue(ctx, routeContextKey{}, route)
}

// RouteFromContext retrieves a route attached by WithRoute. ok is false if
// no route is present, in which case the caller should resolve it itself.
func RouteFromContext(ctx context.Context) (route *Route, ok bool) {
	route, ok = ctx.Value(routeContextKey{}).(*Route)
	return route, ok
}
