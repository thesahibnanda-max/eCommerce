package dumpRepository_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	dumpRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/dump"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	bolt "go.etcd.io/bbolt"
	_ "modernc.org/sqlite"
)

func boltDB(t *testing.T) *bolt.DB {
	t.Helper()
	db, err := bolt.Open(filepath.Join(t.TempDir(), "test.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	err = db.Update(func(tx *bolt.Tx) error {
		users, _ := tx.CreateBucket([]byte("users"))
		_ = users.Put([]byte("u1"), []byte(`{"id":"u1","name":"Asha","password_hash":"abc","password_salt":"xyz"}`))
		_ = users.Put([]byte("u2"), []byte(`["not","an","object"]`))
		index, _ := tx.CreateBucket([]byte("users_by_email"))
		_ = index.Put([]byte("asha@example.com"), []byte("u1"))
		_ = index.Put([]byte("empty"), []byte{})
		blobs, _ := tx.CreateBucket([]byte("blobs"))
		_ = blobs.Put([]byte{0xff, 0x01}, []byte{0x89, 'P', 'N', 'G', 0xff})
		skipped, _ := tx.CreateBucket([]byte("images_content"))
		return skipped.Put([]byte("img"), []byte("huge"))
	})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func find(tables []dumpRepository.Table, name string) (dumpRepository.Table, bool) {
	for _, table := range tables {
		if table.Name == name {
			return table, true
		}
	}
	return dumpRepository.Table{}, false
}

func TestBolt(t *testing.T) {
	tables, err := dumpRepository.Bolt(context.Background(), boltDB(t), dumpRepository.BoltOptions{
		Skip:   []string{"images_content"},
		Redact: []string{"password_hash", "password_salt"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := find(tables, "images_content"); ok {
		t.Fatal("skipped bucket was dumped")
	}
	names := []string{}
	for _, table := range tables {
		names = append(names, table.Name)
	}
	if strings.Join(names, ",") != "blobs,users,users_by_email" {
		t.Fatalf("tables = %v", names)
	}
	users, _ := find(tables, "users")
	if len(users.Rows) != 2 || users.Rows[0].Key != "u1" {
		t.Fatalf("users rows = %+v", users.Rows)
	}
	user, err := utils.FromStringToObjectJSON[map[string]any](users.Rows[0].JSON)
	if err != nil || user["name"] != "Asha" || user["password_hash"] != "[redacted]" || user["password_salt"] != "[redacted]" {
		t.Fatalf("user json = %s (%v)", users.Rows[0].JSON, err)
	}
	if users.Rows[1].JSON != `["not","an","object"]` {
		t.Fatalf("array kept as-is: %s", users.Rows[1].JSON)
	}
	index, _ := find(tables, "users_by_email")
	if index.Rows[0].Key != "asha@example.com" || index.Rows[0].JSON != `"u1"` || index.Rows[1].JSON != `""` {
		t.Fatalf("index rows = %+v", index.Rows)
	}
	blobs, _ := find(tables, "blobs")
	if !strings.HasPrefix(blobs.Rows[0].Key, "base64:") || !strings.Contains(blobs.Rows[0].JSON, `"base64"`) {
		t.Fatalf("binary rows = %+v", blobs.Rows)
	}
}

func sqliteDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, stmt := range []string{
		`CREATE TABLE payments (id TEXT PRIMARY KEY, amount INTEGER, note TEXT, proof_image BLOB, raw BLOB)`,
		`CREATE TABLE payment_events (id INTEGER PRIMARY KEY AUTOINCREMENT, payment_id TEXT, kind TEXT)`,
		`INSERT INTO payments VALUES ('p2', 299, NULL, x'89504e47ff', x'ff00')`,
		`INSERT INTO payments VALUES ('p1', 449, 'first', x'89504e47', CAST('{"a":1}' AS BLOB))`,
		`INSERT INTO payment_events (payment_id, kind) VALUES ('p1', 'created'), ('p1', 'paid')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestSQL(t *testing.T) {
	tables, err := dumpRepository.SQL(context.Background(), sqliteDB(t), []dumpRepository.TableSpec{
		{Name: "payments", Skip: []string{"proof_image"}},
		{Name: "payment_events"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 2 || tables[0].Name != "payments" || tables[1].Name != "payment_events" {
		t.Fatalf("tables = %+v", tables)
	}
	payments := tables[0]
	if len(payments.Rows) != 2 || payments.Rows[0].Key != "p1" || payments.Rows[1].Key != "p2" {
		t.Fatalf("payments ordered by id = %+v", payments.Rows)
	}
	first, err := utils.FromStringToObjectJSON[map[string]any](payments.Rows[0].JSON)
	if err != nil {
		t.Fatal(err)
	}
	if _, has := first["proof_image"]; has {
		t.Fatal("skipped column was dumped")
	}
	if first["amount"] != float64(449) || first["note"] != "first" || first["raw"] != `{"a":1}` {
		t.Fatalf("first payment = %v", first)
	}
	second, _ := utils.FromStringToObjectJSON[map[string]any](payments.Rows[1].JSON)
	if second["note"] != nil || !strings.Contains(payments.Rows[1].JSON, `"base64"`) {
		t.Fatalf("second payment = %s", payments.Rows[1].JSON)
	}
	events := tables[1]
	if len(events.Rows) != 2 || events.Rows[0].Key != "1" || events.Rows[1].Key != "2" {
		t.Fatalf("events = %+v", events.Rows)
	}
}

func TestErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := dumpRepository.Bolt(ctx, boltDB(t), dumpRepository.BoltOptions{}); !errors.Is(err, dumpRepository.ErrCtxInvalid) {
		t.Fatalf("bolt cancelled ctx = %v", err)
	}
	if _, err := dumpRepository.Bolt(context.Background(), nil, dumpRepository.BoltOptions{}); !errors.Is(err, dumpRepository.ErrDatabaseNil) {
		t.Fatalf("nil bolt = %v", err)
	}
	db := sqliteDB(t)
	if _, err := dumpRepository.SQL(context.Background(), db, []dumpRepository.TableSpec{{Name: "payments; DROP TABLE payments"}}); !errors.Is(err, dumpRepository.ErrTableInvalid) {
		t.Fatalf("bad table name = %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE keyless (name TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := dumpRepository.SQL(context.Background(), db, []dumpRepository.TableSpec{{Name: "keyless"}}); err == nil {
		t.Fatal("expected error for table without id")
	}
}
