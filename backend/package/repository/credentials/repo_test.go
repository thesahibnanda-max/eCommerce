package credentialsRepository

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var bg = context.Background()

func open(t *testing.T, dir, key string) Repository {
	t.Helper()
	r, err := NewRepository([]string{dir, "credentials.db"}, 0o600, key)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRoundTripAndEncryptedAtRest(t *testing.T) {
	dir := t.TempDir()
	r := open(t, dir, "admin-key")
	at := time.Date(2026, 11, 28, 10, 0, 0, 0, time.UTC)
	if err := r.Put(bg, Model{Name: "google", Secret: "refresh-token-value", Account: "me@example.com", CreatedAt: at}); err != nil {
		t.Fatal(err)
	}
	got, err := r.Get(bg, "google")
	if err != nil {
		t.Fatal(err)
	}
	if got.Secret != "refresh-token-value" || got.Account != "me@example.com" || !got.CreatedAt.Equal(at) {
		t.Fatalf("got = %+v", got)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "credentials.db"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "refresh-token-value") {
		t.Fatal("secret stored in plain text")
	}
}

func TestWrongKeyCannotDecrypt(t *testing.T) {
	dir := t.TempDir()
	r := open(t, dir, "old-key")
	if err := r.Put(bg, Model{Name: "google", Secret: "s"}); err != nil {
		t.Fatal(err)
	}
	_ = r.Close()
	r = open(t, dir, "new-key")
	defer r.Close()
	if _, err := r.Get(bg, "google"); !errors.Is(err, ErrDecryptFailed) {
		t.Fatalf("err = %v", err)
	}
}

func TestDeleteAndValidation(t *testing.T) {
	r := open(t, t.TempDir(), "k")
	defer r.Close()
	if _, err := r.Get(bg, "google"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing err = %v", err)
	}
	if err := r.Put(bg, Model{Name: "google", Secret: "s"}); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(bg, "google"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Get(bg, "google"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete err = %v", err)
	}
	if err := r.Put(bg, Model{Name: " ", Secret: "s"}); !errors.Is(err, ErrNameInvalid) {
		t.Fatalf("name err = %v", err)
	}
	if err := r.Put(bg, Model{Name: "x"}); !errors.Is(err, ErrSecretRequired) {
		t.Fatalf("secret err = %v", err)
	}
	if _, err := NewRepository([]string{t.TempDir(), "c.db"}, 0o600, " "); !errors.Is(err, ErrKeyRequired) {
		t.Fatalf("key err = %v", err)
	}
	ctx, cancel := context.WithCancel(bg)
	cancel()
	if _, err := r.Get(ctx, "google"); !errors.Is(err, ErrCtxInvalid) {
		t.Fatalf("ctx err = %v", err)
	}
}
