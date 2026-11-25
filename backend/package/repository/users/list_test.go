package usersRepository_test

import (
	"context"
	"errors"
	"testing"

	usersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/users"
)

func TestListUsers(t *testing.T) {
	repo := newRepo(t)
	var ids []string
	for _, name := range []string{"A", "B", "C"} {
		ids = append(ids, mustCreate(t, repo, usersRepository.Model{Name: name}).ID)
	}
	page, total, err := repo.ListUsers(context.Background(), 0, 2)
	if err != nil || total != 3 || len(page) != 2 || page[0].ID != ids[2] || page[1].ID != ids[1] || page[0].Password != "" {
		t.Fatalf("first page: %v total=%d %+v", err, total, page)
	}
	rest, _, err := repo.ListUsers(context.Background(), 2, 2)
	if err != nil || len(rest) != 1 || rest[0].ID != ids[0] {
		t.Fatalf("second page: %v %+v", err, rest)
	}
	if _, _, err := repo.ListUsers(context.Background(), 0, 0); !errors.Is(err, usersRepository.ErrLimitInvalid) {
		t.Fatalf("limit: %v", err)
	}
	if _, _, err := repo.ListUsers(context.Background(), -1, 5); !errors.Is(err, usersRepository.ErrOffsetInvalid) {
		t.Fatalf("offset: %v", err)
	}
}
