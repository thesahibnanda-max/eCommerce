package utils_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func TestGenerateRandomNumberWithLength_InvalidLength(t *testing.T) {
	for _, length := range []uint{0, 129, 1000} {
		for _, onlyPositive := range []bool{true, false} {
			got, err := utils.GenerateRandomNumberWithLength(length, onlyPositive)
			if !errors.Is(err, utils.ErrRandomNumberLength) || got != "" {
				t.Errorf("length %d: got %q, %v; want ErrRandomNumberLength", length, got, err)
			}
		}
	}
}

func TestGenerateRandomNumberWithLength_OnlyPositive(t *testing.T) {
	for _, length := range []uint{1, 6, 19, 20, 64, 128} {
		for i := 0; i < 200; i++ {
			got, err := utils.GenerateRandomNumberWithLength(length, true)
			if err != nil {
				t.Fatalf("length %d: %v", length, err)
			}
			if uint(len(got)) != length || !isDigits(got) {
				t.Fatalf("length %d: got %q, want exactly %d digits", length, got, length)
			}
		}
	}
}

func TestGenerateRandomNumberWithLength_AllowNegative(t *testing.T) {
	const length = 6
	negatives, positives := 0, 0
	for i := 0; i < 2000; i++ {
		got, err := utils.GenerateRandomNumberWithLength(length, false)
		if err != nil {
			t.Fatal(err)
		}
		digits := strings.TrimPrefix(got, "-")
		if len(digits) != length || !isDigits(digits) {
			t.Fatalf("got %q, want an optional '-' followed by exactly %d digits", got, length)
		}
		if strings.HasPrefix(got, "-") {
			if strings.Trim(digits, "0") == "" {
				t.Fatalf("got negative zero %q", got)
			}
			negatives++
		} else {
			positives++
		}
	}
	if negatives < 800 || positives < 800 {
		t.Errorf("sign not balanced: %d negative, %d positive out of 2000", negatives, positives)
	}
}

func TestGenerateRandomNumberWithLength_ZeroNeverNegative(t *testing.T) {
	sawZero := false
	for i := 0; i < 5000; i++ {
		got, err := utils.GenerateRandomNumberWithLength(1, false)
		if err != nil {
			t.Fatal(err)
		}
		if got == "-0" {
			t.Fatal("got negative zero \"-0\"")
		}
		if got == "0" {
			sawZero = true
		}
	}
	if !sawZero {
		t.Error("never generated 0 in 5000 single-digit draws")
	}
}

func TestGenerateRandomNumberWithLength_DigitsUniformIncludingLeadingZero(t *testing.T) {
	const draws = 20000
	first := [10]int{}
	all := [10]int{}
	for i := 0; i < draws; i++ {
		got, err := utils.GenerateRandomNumberWithLength(6, true)
		if err != nil {
			t.Fatal(err)
		}
		first[got[0]-'0']++
		for j := 0; j < len(got); j++ {
			all[got[j]-'0']++
		}
	}
	for d := 0; d < 10; d++ {
		if first[d] < draws/10*7/10 || first[d] > draws/10*13/10 {
			t.Errorf("first digit %d appeared %d times, want about %d (leading zeros must be possible)", d, first[d], draws/10)
		}
		if all[d] < draws*6/10*8/10 || all[d] > draws*6/10*12/10 {
			t.Errorf("digit %d appeared %d times overall, want about %d", d, all[d], draws*6/10)
		}
	}
}

func TestGenerateRandomNumberWithLength_Unique(t *testing.T) {
	seen := make(map[string]struct{})
	for i := 0; i < 1000; i++ {
		got, err := utils.GenerateRandomNumberWithLength(20, true)
		if err != nil {
			t.Fatal(err)
		}
		if _, dup := seen[got]; dup {
			t.Fatalf("duplicate 20-digit number %q", got)
		}
		seen[got] = struct{}{}
	}
}
