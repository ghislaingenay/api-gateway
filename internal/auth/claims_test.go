package auth

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestCustomClaims_Validate(t *testing.T) {
	tests := []struct {
		name    string
		claims  CustomClaims
		wantErr bool
	}{
		{
			name:    "valid claims",
			claims:  validClaims(),
			wantErr: false,
		},
		{
			name: "missing tenant_id",
			claims: func() CustomClaims {
				c := validClaims()
				c.TenantID = uuid.Nil
				return c
			}(),
			wantErr: true,
		},
		{
			name: "missing user_id",
			claims: func() CustomClaims {
				c := validClaims()
				c.UserID = uuid.Nil
				return c
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.claims.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrMissingClaims) {
				t.Errorf("Validate() error = %v, want wrapped ErrMissingClaims", err)
			}
		})
	}
}
