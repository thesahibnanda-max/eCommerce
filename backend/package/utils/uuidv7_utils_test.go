package utils_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

func TestNewUUIDv7(t *testing.T) {
	previous := ""
	for i := 0; i < 1000; i++ {
		id, err := utils.NewUUIDv7()
		if err != nil {
			t.Fatalf("NewUUIDv7() error = %v", err)
		}
		if !utils.ValidateUUIDv7(id) {
			t.Fatalf("ValidateUUIDv7(%q) = false", id)
		}
		if id <= previous {
			t.Fatalf("NewUUIDv7() not increasing: %q after %q", id, previous)
		}
		previous = id
	}
}

func TestNewUUIDv7_Concurrent(t *testing.T) {
	const workers, each = 8, 500
	var mu sync.Mutex
	seen := make(map[string]struct{}, workers*each)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < each; i++ {
				id, err := utils.NewUUIDv7()
				if err != nil {
					t.Error(err)
					return
				}
				mu.Lock()
				seen[id] = struct{}{}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(seen) != workers*each {
		t.Fatalf("got %d unique ids, want %d", len(seen), workers*each)
	}
}

func TestValidateUUIDv7(t *testing.T) {
	valid, err := utils.NewUUIDv7()
	if err != nil {
		t.Fatalf("NewUUIDv7() error = %v", err)
	}
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{name: "generated", id: valid, want: true},
		{name: "rfc 9562 example", id: "017f22e2-79b0-7cc3-98c4-dc0c0c07398f", want: true},
		{name: "uppercase", id: strings.ToUpper(valid), want: true},
		{name: "empty", id: ""},
		{name: "random text", id: "not-a-uuid"},
		{name: "v4", id: "919108f7-52d1-4320-9bac-f847db4148a8"},
		{name: "v1", id: "c232ab00-9414-11ec-b3c8-9f6bdeced846"},
		{name: "nil uuid", id: "00000000-0000-0000-0000-000000000000"},
		{name: "max uuid", id: "ffffffff-ffff-ffff-ffff-ffffffffffff"},
		{name: "wrong variant", id: "017f22e2-79b0-7cc3-c8c4-dc0c0c07398f"},
		{name: "no hyphens", id: strings.ReplaceAll(valid, "-", "")},
		{name: "braces", id: "{" + valid + "}"},
		{name: "urn", id: "urn:uuid:" + valid},
		{name: "surrounding spaces", id: " " + valid + " "},
		{name: "bad hex", id: "g" + valid[1:]},
		{name: "too short", id: valid[:35]},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := utils.ValidateUUIDv7(tt.id); got != tt.want {
				t.Fatalf("ValidateUUIDv7(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}
