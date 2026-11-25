package usersRepository

import (
	"encoding/hex"
	"testing"
)

func TestPasswordHashing(t *testing.T) {
	salt1, err := newPasswordSalt()
	if err != nil {
		t.Fatal(err)
	}
	salt2, _ := newPasswordSalt()
	if salt1 == salt2 {
		t.Error("two salts are identical")
	}
	if raw, err := hex.DecodeString(salt1); err != nil || len(raw) != passwordSaltBytes {
		t.Errorf("salt %q is not %d hex-encoded bytes", salt1, passwordSaltBytes)
	}

	h1 := hashPassword(salt1, "Secr3t!pass")
	if len(h1) != 64 {
		t.Errorf("hash length = %d, want 64 hex characters of SHA-256", len(h1))
	}
	if h1 != hashPassword(salt1, "Secr3t!pass") {
		t.Error("hashing is not deterministic for the same salt")
	}
	if h1 == hashPassword(salt2, "Secr3t!pass") {
		t.Error("same password with different salts gives the same hash")
	}

	if !verifyPassword(salt1, h1, "Secr3t!pass") {
		t.Error("correct password rejected")
	}
	for _, wrong := range []string{"", "Secr3t!pas", "Secr3t!pass ", "secr3t!pass"} {
		if verifyPassword(salt1, h1, wrong) {
			t.Errorf("wrong password %q accepted", wrong)
		}
	}
	if verifyPassword(salt2, h1, "Secr3t!pass") {
		t.Error("correct password accepted with the wrong salt")
	}
}
