package cartsRepository

import (
	"context"
	"errors"
	"testing"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

func TestListCarts(t *testing.T) {
	r := newTestRepo(t)
	users := []string{utils.NewULID(), utils.NewULID()}
	for _, user := range users {
		if _, err := r.UpdateCart(context.Background(), user, addItem(utils.NewULID(), 2)); err != nil {
			t.Fatalf("UpdateCart: %v", err)
		}
	}
	carts, total, err := r.ListCarts(context.Background(), 0, 10)
	if err != nil || total != 2 || len(carts) != 2 || carts[0].UserID != users[1] || len(carts[0].Items) != 1 {
		t.Fatalf("list: %v total=%d %+v", err, total, carts)
	}
	if _, _, err := r.ListCarts(context.Background(), 0, 101); !errors.Is(err, ErrLimitInvalid) {
		t.Fatalf("limit: %v", err)
	}
}
