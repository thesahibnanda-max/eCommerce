package usersRepository_test

import (
	"context"
	"strings"
	"testing"

	usersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/users"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

func TestDump_RedactsPasswords(t *testing.T) {
	repo := newRepo(t)
	created := mustCreate(t, repo, usersRepository.Model{Name: "Asha Rao"})
	tables, err := repo.Dump(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	var found bool
	for _, table := range tables {
		names = append(names, table.Name)
		if table.Name != "users" {
			continue
		}
		for _, row := range table.Rows {
			if row.Key != created.ID {
				continue
			}
			found = true
			record, err := utils.FromStringToObjectJSON[map[string]any](row.JSON)
			if err != nil || record["password_hash"] != "[redacted]" || record["password_salt"] != "[redacted]" || record["name"] != "Asha Rao" {
				t.Fatalf("user record = %s (%v)", row.JSON, err)
			}
		}
	}
	if !found || !strings.Contains(strings.Join(names, ","), "users_by_email") {
		t.Fatalf("tables = %v found=%v", names, found)
	}
}
