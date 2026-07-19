package auth

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// JWTAuthMiddleware validates the Authorization: Bearer <jwt> header on
// every request against an explicit signing-algorithm allowlist, resolves
// the signing key via the token's kid through keyStore, and attaches
// CustomClaims to the request context for downstream middleware.
//
// Passing allowedAlgorithms explicitly to the underlying parser (rather than
// trusting the token's own alg header) is what prevents algorithm confusion
// attacks, including alg=none.
func JWTAuthMiddleware(keyStore KeyStore, allowedAlgorithms []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString, err := bearerToken(r)
			if err != nil {
				log.Printf("jwt auth rejected: %v", err)
				writeUnauthorized(w)
				return
			}

			claims := &CustomClaims{}
			_, err = jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				kid, ok := token.Header["kid"].(string)
				if !ok || kid == "" {
					return nil, ErrUnknownKey
				}
				return keyStore.GetKey(kid)
			}, jwt.WithValidMethods(allowedAlgorithms))
			if err != nil {
				log.Printf("jwt auth rejected: %v", err)
				writeUnauthorized(w)
				return
			}

			next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
		})
	}
}

func bearerToken(r *http.Request) (string, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", ErrMissingToken
	}
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", ErrMalformedToken
	}
	return token, nil
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"error":   "unauthorized",
		"message": "invalid or missing token",
	}); err != nil {
		log.Printf("failed to write unauthorized response: %v", err)
	}
}
