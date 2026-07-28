// Command mockjwks is a minimal local JWKS publisher for localhost/
// docker-compose development. The gateway signs tokens itself
// (JWT_SIGNING_KID/JWT_SIGNING_PRIVATE_KEY) but has no JWKS-publishing
// side of its own (TD-011 scoped that out as an auth/identity server's
// responsibility). This command fills that gap for local dev only: it
// derives the public key from the same signing key already in .env and
// serves it as a JWKS document, so JWT_JWKS_URL has something to fetch.
package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"api-gateway/internal/logger"

	"github.com/MicahParks/jwkset"
	"github.com/golang-jwt/jwt/v5"
)

func main() {
	port := "8083"

	kid := strings.TrimSpace(os.Getenv("JWT_SIGNING_KID"))
	privateKeyPEMBase64 := strings.TrimSpace(os.Getenv("JWT_SIGNING_PRIVATE_KEY"))
	if kid == "" || privateKeyPEMBase64 == "" {
		logger.Default().Error("mockjwks: JWT_SIGNING_KID and JWT_SIGNING_PRIVATE_KEY must be set")
		os.Exit(1)
	}

	pemBytes, err := base64.StdEncoding.DecodeString(privateKeyPEMBase64)
	if err != nil {
		logger.Default().Error("mockjwks: decode JWT_SIGNING_PRIVATE_KEY", "error", err.Error())
		os.Exit(1)
	}
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(pemBytes)
	if err != nil {
		logger.Default().Error("mockjwks: parse JWT_SIGNING_PRIVATE_KEY", "error", err.Error())
		os.Exit(1)
	}

	jwk, err := jwkset.NewJWKFromKey(&privateKey.PublicKey, jwkset.JWKOptions{
		Metadata: jwkset.JWKMetadataOptions{KID: kid, ALG: jwkset.AlgRS256},
	})
	if err != nil {
		logger.Default().Error("mockjwks: build JWK", "error", err.Error())
		os.Exit(1)
	}
	body, err := json.Marshal(jwkset.JWKSMarshal{Keys: []jwkset.JWKMarshal{jwk.Marshal()}})
	if err != nil {
		logger.Default().Error("mockjwks: marshal JWKS", "error", err.Error())
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})

	logger.Default().Info("mockjwks: listening", "port", port, "kid", kid)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		logger.Default().Error("mockjwks: server stopped", "error", err.Error())
		os.Exit(1)
	}
}
