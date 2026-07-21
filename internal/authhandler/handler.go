package authhandler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"api-gateway/internal/auth"
	"api-gateway/internal/rbac"
	"api-gateway/internal/refreshtoken"
	"api-gateway/internal/tenant"
	"api-gateway/internal/user"
	"api-gateway/internal/validation"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 7 * 24 * time.Hour
)

// LoginHandler returns an http.HandlerFunc for POST /auth/login.
func LoginHandler(users user.Repository, tenants tenant.Repository, refreshTokens refreshtoken.Repository, roles rbac.RoleCache, signer auth.Signer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "malformed request body")
			return
		}
		if err := validation.Validate(req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "email, password, and tenant_slug are required")
			return
		}

		ctx := r.Context()

		t, err := tenants.GetBySlug(ctx, req.TenantSlug)
		if err != nil {
			writeInvalidCredentials(w)
			return
		}

		u, err := users.GetByEmail(ctx, t.ID, req.Email)
		if err != nil {
			writeInvalidCredentials(w)
			return
		}
		if !u.IsActive {
			writeInvalidCredentials(w)
			return
		}
		if err := auth.ComparePassword(u.PasswordHash, req.Password); err != nil {
			writeInvalidCredentials(w)
			return
		}

		role, ok := roles.GetRoleByID(u.RoleID)
		if !ok {
			log.Printf("authhandler: login: user %s has unknown role_id %s", u.ID, u.RoleID)
			writeError(w, http.StatusInternalServerError, "internal_error", "could not resolve user role")
			return
		}

		resp, err := issueTokenPair(ctx, refreshTokens, signer, *u, *role)
		if err != nil {
			log.Printf("authhandler: login: issue tokens for user %s: %v", u.ID, err)
			writeError(w, http.StatusInternalServerError, "internal_error", "could not issue tokens")
			return
		}

		if err := users.UpdateLastLoginAt(ctx, u.ID, time.Now()); err != nil {
			log.Printf("authhandler: login: update last_login_at for user %s: %v", u.ID, err)
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

// RefreshHandler returns an http.HandlerFunc for POST /auth/refresh.
func RefreshHandler(refreshTokens refreshtoken.Repository, users user.Repository, roles rbac.RoleCache, signer auth.Signer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "malformed request body")
			return
		}
		if err := validation.Validate(req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "refresh_token is required")
			return
		}

		ctx := r.Context()

		existing, err := refreshTokens.GetByHash(ctx, auth.HashRefreshToken(req.RefreshToken))
		if err != nil {
			writeInvalidToken(w)
			return
		}
		if !existing.Valid(time.Now()) {
			writeInvalidToken(w)
			return
		}

		u, err := users.GetByID(ctx, existing.UserID)
		if err != nil || !u.IsActive {
			writeInvalidToken(w)
			return
		}

		role, ok := roles.GetRoleByID(u.RoleID)
		if !ok {
			log.Printf("authhandler: refresh: user %s has unknown role_id %s", u.ID, u.RoleID)
			writeError(w, http.StatusInternalServerError, "internal_error", "could not resolve user role")
			return
		}

		if err := refreshTokens.Revoke(ctx, existing.ID); err != nil {
			log.Printf("authhandler: refresh: revoke old token for user %s: %v", u.ID, err)
			writeError(w, http.StatusInternalServerError, "internal_error", "could not rotate refresh token")
			return
		}

		resp, err := issueTokenPair(ctx, refreshTokens, signer, *u, *role)
		if err != nil {
			log.Printf("authhandler: refresh: issue tokens for user %s: %v", u.ID, err)
			writeError(w, http.StatusInternalServerError, "internal_error", "could not issue tokens")
			return
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

// LogoutHandler returns an http.HandlerFunc for POST /auth/logout. It
// requires a valid access token (the route must be wrapped with JWT auth)
// and is idempotent: revoking an already-revoked or unknown refresh token
// still returns 200.
func LogoutHandler(refreshTokens refreshtoken.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req LogoutRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "malformed request body")
			return
		}
		if err := validation.Validate(req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "refresh_token is required")
			return
		}

		ctx := r.Context()
		existing, err := refreshTokens.GetByHash(ctx, auth.HashRefreshToken(req.RefreshToken))
		if err == nil {
			if err := refreshTokens.Revoke(ctx, existing.ID); err != nil {
				log.Printf("authhandler: logout: revoke token: %v", err)
			}
		} else if !errors.Is(err, refreshtoken.ErrNotFound) {
			log.Printf("authhandler: logout: lookup token: %v", err)
		}

		writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
	}
}

// MeHandler returns an http.HandlerFunc for GET /auth/me. It requires a
// valid access token and refetches the user for freshness (catching
// deactivation since the token was issued, unlike serving straight from
// claims).
func MeHandler(users user.Repository, roles rbac.RoleCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok || claims == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or missing token")
			return
		}

		u, err := users.GetByID(r.Context(), claims.UserID)
		if err != nil || !u.IsActive {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or missing token")
			return
		}

		role, ok := roles.GetRoleByID(u.RoleID)
		roleName := claims.Role
		if ok {
			roleName = role.Name
		}

		writeJSON(w, http.StatusOK, newUserResponse(*u, roleName))
	}
}

// issueTokenPair signs a new access token and generates+stores a new
// refresh token for u.
func issueTokenPair(ctx context.Context, refreshTokens refreshtoken.Repository, signer auth.Signer, u user.User, role rbac.Role) (LoginResponse, error) {
	now := time.Now()
	claims := auth.CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID.String(),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
		TenantID:    u.TenantID,
		UserID:      u.ID,
		Role:        role.Name,
		RoleID:      role.ID,
		Permissions: role.Permissions,
		Email:       u.Email,
	}

	accessToken, err := signer.Sign(claims)
	if err != nil {
		return LoginResponse{}, err
	}

	rawRefresh, hashedRefresh, err := auth.GenerateRefreshToken()
	if err != nil {
		return LoginResponse{}, err
	}

	if err := refreshTokens.Create(ctx, refreshtoken.RefreshToken{
		ID:        uuid.New(),
		UserID:    u.ID,
		TokenHash: hashedRefresh,
		ExpiresAt: now.Add(refreshTokenTTL),
	}); err != nil {
		return LoginResponse{}, err
	}

	return LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresIn:    int(accessTokenTTL.Seconds()),
		TokenType:    "Bearer",
	}, nil
}

func writeInvalidCredentials(w http.ResponseWriter) {
	writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
}

func writeInvalidToken(w http.ResponseWriter) {
	writeError(w, http.StatusUnauthorized, "invalid_token", "invalid or expired refresh token")
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("authhandler: failed to write response: %v", err)
	}
}
