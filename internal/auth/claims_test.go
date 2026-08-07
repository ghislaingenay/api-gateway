package auth

import (
	"errors"
	"testing"
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
			name: "missing sub",
			claims: func() CustomClaims {
				c := validClaims()
				c.Subject = ""
				return c
			}(),
			wantErr: true,
		},
		{
			name: "missing email",
			claims: func() CustomClaims {
				c := validClaims()
				c.Email = ""
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
