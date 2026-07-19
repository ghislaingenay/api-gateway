package auth

import "context"

// allows your application to pass user identity information (like user IDs or roles) down through different layers (handlers, database logic, etc.)
// without having to pass the data as an explicit argument to every function.
type contextKey int

const claimsContextKey contextKey = iota

// WithClaims returns a new context carrying the given validated claims.
func WithClaims(ctx context.Context, claims *CustomClaims) context.Context {
	return context.WithValue(ctx, claimsContextKey, claims)
}

// ClaimsFromContext retrieves the validated claims attached by
// JWTAuthMiddleware. ok is false if no claims are present.
func ClaimsFromContext(ctx context.Context) (claims *CustomClaims, ok bool) {
	claims, ok = ctx.Value(claimsContextKey).(*CustomClaims)
	return claims, ok
}
