package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"api-gateway/config"
	"api-gateway/internal/clientip"
	"api-gateway/internal/loginguard"
	"api-gateway/internal/logger"
	"api-gateway/internal/rbac"
	"api-gateway/internal/refreshtoken"
	"api-gateway/internal/resilience"
	"api-gateway/internal/rules"
	"api-gateway/internal/tenant"
	"api-gateway/internal/user"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 7 * 24 * time.Hour
)

// LoginHandler returns an http.HandlerFunc for POST /auth/login. Failed
// attempts are progressively delayed per caller (guard, keyed by IP + tenant
// + email) to slow down credential-stuffing and brute-force attempts; the
// counter resets on a successful login.
func LoginHandler(users user.Repository, tenants tenant.Repository, refreshTokens refreshtoken.Repository, roles rbac.RoleCache, signer Signer, guard loginguard.Guard, delayCfg config.LoginSecurityConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_request", "malformed request body")
			return
		}
		if err := rules.Validate(req); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_request", "email, password, and tenant_slug are required")
			return
		}

		ctx := r.Context()
		failureKey := clientip.FromRequest(r) + ":" + req.TenantSlug + ":" + req.Email

		fail := func() {
			delayOnFailure(ctx, guard, delayCfg, failureKey)
			writeInvalidCredentials(w, r)
		}

		t, err := tenants.GetBySlug(ctx, req.TenantSlug)
		if err != nil {
			fail()
			return
		}

		u, err := users.GetByEmail(ctx, t.ID, req.Email)
		if err != nil {
			fail()
			return
		}
		if !u.IsActive {
			fail()
			return
		}
		if err := ComparePassword(u.PasswordHash, req.Password); err != nil {
			fail()
			return
		}

		role, ok := roles.GetRoleByID(u.RoleID)
		if !ok {
			logger.FromContext(ctx).Error("authhandler: login: user has unknown role_id", "user_id", u.ID.String(), "role_id", u.RoleID.String())
			writeError(w, r, http.StatusInternalServerError, "internal_error", "could not resolve user role")
			return
		}

		resp, err := issueTokenPair(ctx, refreshTokens, signer, *u, *role)
		if err != nil {
			logger.FromContext(ctx).Error("authhandler: login: issue tokens", "user_id", u.ID.String(), "error", err.Error())
			writeError(w, r, http.StatusInternalServerError, "internal_error", "could not issue tokens")
			return
		}

		if err := users.UpdateLastLoginAt(ctx, u.ID, time.Now()); err != nil {
			logger.FromContext(ctx).Warn("authhandler: login: update last_login_at", "user_id", u.ID.String(), "error", err.Error())
		}

		if err := guard.Reset(ctx, failureKey); err != nil {
			logger.FromContext(ctx).Warn("authhandler: login: reset failure guard", "error", err.Error())
		}

		writeJSON(w, r, http.StatusOK, resp)
	}
}

// delayOnFailure records a failed login attempt and blocks for a jittered,
// exponentially increasing delay (capped at delayCfg.DelayMax) before the
// caller returns its 401 response. It fails open (no delay) if the guard
// can't be reached, and returns early if ctx is cancelled mid-sleep.
func delayOnFailure(ctx context.Context, guard loginguard.Guard, delayCfg config.LoginSecurityConfig, key string) {
	attempt, err := guard.RegisterFailure(ctx, key)
	if err != nil {
		logger.FromContext(ctx).Warn("authhandler: login: register failure", "error", err.Error())
		return
	}

	delay := (resilience.RetryPolicy{BaseBackoff: delayCfg.DelayBaseBackoff}).Backoff(attempt)
	if delay > delayCfg.DelayMax {
		delay = delayCfg.DelayMax
	}
	if delay <= 0 {
		return
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}

// RefreshHandler returns an http.HandlerFunc for POST /auth/refresh.
func RefreshHandler(refreshTokens refreshtoken.Repository, users user.Repository, roles rbac.RoleCache, signer Signer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_request", "malformed request body")
			return
		}
		if err := rules.Validate(req); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_request", "refresh_token is required")
			return
		}

		ctx := r.Context()

		existing, err := refreshTokens.GetByHash(ctx, HashRefreshToken(req.RefreshToken))
		if err != nil {
			writeInvalidToken(w, r)
			return
		}
		if !existing.Valid(time.Now()) {
			writeInvalidToken(w, r)
			return
		}

		u, err := users.GetByID(ctx, existing.UserID)
		if err != nil || !u.IsActive {
			writeInvalidToken(w, r)
			return
		}

		role, ok := roles.GetRoleByID(u.RoleID)
		if !ok {
			logger.FromContext(ctx).Error("authhandler: refresh: user has unknown role_id", "user_id", u.ID.String(), "role_id", u.RoleID.String())
			writeError(w, r, http.StatusInternalServerError, "internal_error", "could not resolve user role")
			return
		}

		if err := refreshTokens.Revoke(ctx, existing.ID); err != nil {
			logger.FromContext(ctx).Error("authhandler: refresh: revoke old token", "user_id", u.ID.String(), "error", err.Error())
			writeError(w, r, http.StatusInternalServerError, "internal_error", "could not rotate refresh token")
			return
		}

		resp, err := issueTokenPair(ctx, refreshTokens, signer, *u, *role)
		if err != nil {
			logger.FromContext(ctx).Error("authhandler: refresh: issue tokens", "user_id", u.ID.String(), "error", err.Error())
			writeError(w, r, http.StatusInternalServerError, "internal_error", "could not issue tokens")
			return
		}

		writeJSON(w, r, http.StatusOK, resp)
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
			writeError(w, r, http.StatusBadRequest, "invalid_request", "malformed request body")
			return
		}
		if err := rules.Validate(req); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_request", "refresh_token is required")
			return
		}

		ctx := r.Context()
		existing, err := refreshTokens.GetByHash(ctx, HashRefreshToken(req.RefreshToken))
		if err == nil {
			if err := refreshTokens.Revoke(ctx, existing.ID); err != nil {
				logger.FromContext(ctx).Error("authhandler: logout: revoke token", "error", err.Error())
			}
		} else if !errors.Is(err, refreshtoken.ErrNotFound) {
			logger.FromContext(ctx).Error("authhandler: logout: lookup token", "error", err.Error())
		}

		writeJSON(w, r, http.StatusOK, map[string]string{"message": "logged out"})
	}
}

// MeHandler returns an http.HandlerFunc for GET /auth/me. It requires a
// valid access token and refetches the user for freshness (catching
// deactivation since the token was issued, unlike serving straight from
// claims).
func MeHandler(users user.Repository, roles rbac.RoleCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok || claims == nil {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "invalid or missing token")
			return
		}

		u, err := users.GetByID(r.Context(), claims.UserID)
		if err != nil || !u.IsActive {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "invalid or missing token")
			return
		}

		role, ok := roles.GetRoleByID(u.RoleID)
		roleName := claims.Role
		if ok {
			roleName = role.Name
		}

		writeJSON(w, r, http.StatusOK, newUserResponse(*u, roleName))
	}
}

// issueTokenPair signs a new access token and generates+stores a new
// refresh token for u.
func issueTokenPair(ctx context.Context, refreshTokens refreshtoken.Repository, signer Signer, u user.User, role rbac.Role) (LoginResponse, error) {
	now := time.Now()
	claims := CustomClaims{
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

	rawRefresh, hashedRefresh, err := GenerateRefreshToken()
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

func writeInvalidCredentials(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
}

func writeInvalidToken(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, http.StatusUnauthorized, "invalid_token", "invalid or expired refresh token")
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, r, status, map[string]string{"error": code, "message": message})
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		logger.FromContext(r.Context()).Error("authhandler: failed to write response", "error", err.Error())
	}
}
