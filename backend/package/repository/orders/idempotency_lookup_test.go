package ordersRepository

import (
	"context"
	"errors"
	"testing"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

func TestGetUserOrderByIdempotencyKey(t *testing.T) {
	r := newRepo(t)
	user := utils.NewULID()
	in := input(user)
	created, err := r.CreateOrder(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	got, err := r.GetUserOrderByIdempotencyKey(context.Background(), user, " "+in.IdempotencyKey+" ")
	if err != nil || got.ID != created.ID || len(got.Items) != len(created.Items) {
		t.Fatalf("lookup: %v", err)
	}
	if _, err := r.GetUserOrderByIdempotencyKey(context.Background(), utils.NewULID(), in.IdempotencyKey); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("other user: %v", err)
	}
	if _, err := r.GetUserOrderByIdempotencyKey(context.Background(), user, ""); !errors.Is(err, ErrIdempotencyKeyInvalid) {
		t.Fatalf("empty key: %v", err)
	}
	if _, err := r.GetUserOrderByIdempotencyKey(context.Background(), "bad", "k"); !errors.Is(err, ErrUserIDInvalid) {
		t.Fatalf("bad user: %v", err)
	}
}
