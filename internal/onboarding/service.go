package onboarding

import (
	"context"
	"database/sql"
	"fmt"

	"api-gateway/internal/logger"

	"github.com/google/uuid"
)

// ownerRoleName is the system role automatically granted to whoever
// creates a tenant via Onboard.
const ownerRoleName = "owner"

// Service creates a tenant and makes the calling user its owner.
type Service interface {
	// Onboard creates a tenants row and a tenant_users row (role=owner) for
	// userID in a single transaction, returning the new tenant's id.
	Onboard(ctx context.Context, userID uuid.UUID, organizationName string) (tenantID uuid.UUID, err error)
}

type service struct {
	db *sql.DB
}

// NewService returns a Service backed by PostgreSQL.
func NewService(db *sql.DB) Service {
	return &service{db: db}
}

// Onboard implements Service.
func (s *service) Onboard(ctx context.Context, userID uuid.UUID, organizationName string) (uuid.UUID, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin onboarding transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			logger.FromContext(ctx).Error("onboarding: rollback transaction", "error", rbErr.Error())
		}
	}()

	var tenantID uuid.UUID
	err = tx.QueryRowContext(ctx, `
		INSERT INTO tenants (name, slug)
		VALUES ($1, $2)
		RETURNING id
	`, organizationName, uuid.New().String()).Scan(&tenantID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert tenant: %w", err)
	}

	var ownerRoleID uuid.UUID
	if err := tx.QueryRowContext(ctx, `SELECT id FROM roles WHERE name = $1`, ownerRoleName).Scan(&ownerRoleID); err != nil {
		return uuid.Nil, fmt.Errorf("lookup owner role: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO tenant_users (tenant_id, user_id, role_id)
		VALUES ($1, $2, $3)
	`, tenantID, userID, ownerRoleID); err != nil {
		return uuid.Nil, fmt.Errorf("insert tenant_users: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return uuid.Nil, fmt.Errorf("commit onboarding transaction: %w", err)
	}

	return tenantID, nil
}
