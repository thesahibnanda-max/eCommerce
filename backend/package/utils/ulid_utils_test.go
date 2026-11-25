package utils_test

import (
	"strings"
	"testing"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

func TestValidateULID(t *testing.T) {
	valid := utils.NewULID()

	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{name: "valid ulid", id: valid},
		{name: "lowercase ulid", id: strings.ToLower(valid)},
		{name: "empty", id: "", wantErr: true},
		{name: "random text", id: "not-a-ulid", wantErr: true},
		{name: "too short", id: valid[:25], wantErr: true},
		{name: "too long", id: valid + "0", wantErr: true},
		{name: "invalid crockford character", id: "0" + strings.Repeat("U", 25), wantErr: true},
		{name: "overflowing timestamp", id: "8" + strings.Repeat("0", 25), wantErr: true},
		{name: "surrounding spaces", id: " " + valid + " ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := utils.ValidateULID(tt.id); got == tt.wantErr {
				t.Errorf("ValidateULID(%q) = %v, want %v", tt.id, got, !tt.wantErr)
			}
		})
	}
}

func TestNewULID(t *testing.T) {
	const n = 1000
	seen := make(map[string]struct{}, n)
	prev := ""
	for i := 0; i < n; i++ {
		id := utils.NewULID()
		if len(id) != 26 {
			t.Fatalf("NewULID() = %q, want 26 characters", id)
		}
		if !utils.ValidateULID(id) {
			t.Fatalf("NewULID() = %q is not a valid ULID", id)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("NewULID() returned duplicate %q", id)
		}
		if id <= prev {
			t.Fatalf("NewULID() = %q not after %q, want increasing", id, prev)
		}
		seen[id] = struct{}{}
		prev = id
	}
}
