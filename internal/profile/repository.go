package profile

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Repository loads profile records from persistent storage.
type Repository interface {
	GetByUserID(ctx context.Context, userID uuid.UUID) (*Profile, error)
}

type postgresRepository struct {
	db *sql.DB
}

// NewRepository returns a Repository backed by PostgreSQL.
func NewRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

const selectProfileColumns = `
	id, user_id, first_name, last_name, avatar_url, timezone, metadata, created_at, updated_at
`

// GetByUserID implements Repository.
func (r *postgresRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*Profile, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+selectProfileColumns+`
		FROM profiles
		WHERE user_id = $1
	`, userID)

	var (
		p            Profile
		metadataJSON []byte
	)
	err := row.Scan(&p.ID, &p.UserID, &p.FirstName, &p.LastName, &p.AvatarURL, &p.Timezone, &metadataJSON, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProfileNotFound
		}
		return nil, fmt.Errorf("query profile: %w", err)
	}
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &p.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal profile metadata: %w", err)
		}
	}
	return &p, nil
}
