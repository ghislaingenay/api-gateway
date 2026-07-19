package rbac

import (
	"reflect"
	"testing"
)

func TestUnmarshalPermissions(t *testing.T) {
	tests := []struct {
		name    string
		raw     []byte
		want    []string
		wantErr bool
	}{
		{"nil input yields nil", nil, nil, false},
		{"empty input yields nil", []byte{}, nil, false},
		{"valid json array", []byte(`["users:read", "users:create"]`), []string{"users:read", "users:create"}, false},
		{"empty json array", []byte(`[]`), []string{}, false},
		{"malformed json returns error", []byte(`not json`), nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := unmarshalPermissions(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("unmarshalPermissions() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("unmarshalPermissions() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
