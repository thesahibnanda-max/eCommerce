package utils_test

import (
	"testing"
	"time"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

func TestTimeNow(t *testing.T) {
	before := time.Now()
	first, err := utils.TimeNow()
	if err != nil {
		t.Fatalf("TimeNow: %v", err)
	}
	second, err := utils.TimeNow()
	if err != nil {
		t.Fatalf("TimeNow: %v", err)
	}
	after := time.Now()

	if first.Before(before) || second.After(after) || second.Before(first) {
		t.Fatalf("TimeNow values %v, %v not within %v .. %v", first, second, before, after)
	}
	if first.Location().String() != second.Location().String() {
		t.Fatalf("location changed between calls: %v vs %v", first.Location(), second.Location())
	}
	if first.Location() == time.Local && time.Local.String() == "Local" {
		t.Fatalf("TimeNow must use a fixed zone, not the machine's local zone")
	}
}
