package identity

import "context"

// contextKey is unexported so only this package's WithIdentity/FromContext
// pair can set or read the value, mirroring auth.WithClaims/ClaimsFromContext.
type contextKey int

const identityContextKey contextKey = iota

// WithIdentity returns a new context carrying the given resolved identity.
func WithIdentity(ctx context.Context, identity *ResolvedIdentity) context.Context {
	return context.WithValue(ctx, identityContextKey, identity)
}

// FromContext retrieves the ResolvedIdentity attached by ResolveMiddleware.
// ok is false if none is present.
func IndentityFromContext(ctx context.Context) (identity *ResolvedIdentity, ok bool) {
	identity, ok = ctx.Value(identityContextKey).(*ResolvedIdentity)
	return identity, ok
}
