package cartsRepository

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

var bg = context.Background()

func newTestRepo(t *testing.T) *repository {
	t.Helper()
	r, err := NewRepository([]string{t.TempDir(), "carts.db"}, 0o600)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r.(*repository)
}

func addItem(variantID string, quantity int) func(Model) (Model, error) {
	return func(m Model) (Model, error) {
		m.Items = append(m.Items, Item{VariantID: variantID, Quantity: quantity, AddedAt: time.Unix(1, 0)})
		return m, nil
	}
}

func Test_GetCart_Empty(t *testing.T) {
	r := newTestRepo(t)
	user := utils.NewULID()
	cart, err := r.GetCart(bg, user)
	if err != nil {
		t.Fatalf("GetCart() error = %v", err)
	}
	if cart.UserID != user || cart.Items == nil || len(cart.Items) != 0 {
		t.Fatalf("GetCart() = %+v, want empty cart for user", cart)
	}
}

func Test_UpdateCart_PersistsAndStamps(t *testing.T) {
	r := newTestRepo(t)
	user := utils.NewULID()
	variant := utils.NewULID()
	stamp := time.Unix(1_700_000_000, 0)
	r.now = func() (time.Time, error) { return stamp, nil }
	updated, err := r.UpdateCart(bg, user, addItem(variant, 2))
	if err != nil {
		t.Fatalf("UpdateCart() error = %v", err)
	}
	if !updated.UpdatedAt.Equal(stamp) || updated.UserID != user {
		t.Fatalf("UpdateCart() = %+v", updated)
	}
	got, err := r.GetCart(bg, user)
	if err != nil {
		t.Fatalf("GetCart() error = %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].VariantID != variant || got.Items[0].Quantity != 2 || !got.UpdatedAt.Equal(stamp) {
		t.Fatalf("GetCart() = %+v", got)
	}
}

func Test_UpdateCart_IgnoresUserIDChange(t *testing.T) {
	r := newTestRepo(t)
	user := utils.NewULID()
	updated, err := r.UpdateCart(bg, user, func(m Model) (Model, error) {
		m.UserID = utils.NewULID()
		m.Items = []Item{{VariantID: utils.NewULID(), Quantity: 1}}
		return m, nil
	})
	if err != nil || updated.UserID != user {
		t.Fatalf("UpdateCart() = %+v, %v", updated, err)
	}
}

func Test_UpdateCart_EmptyDeletes(t *testing.T) {
	r := newTestRepo(t)
	user := utils.NewULID()
	if _, err := r.UpdateCart(bg, user, addItem(utils.NewULID(), 1)); err != nil {
		t.Fatal(err)
	}
	if _, err := r.UpdateCart(bg, user, func(m Model) (Model, error) {
		m.Items = nil
		return m, nil
	}); err != nil {
		t.Fatal(err)
	}
	got, _ := r.GetCart(bg, user)
	if len(got.Items) != 0 || !got.UpdatedAt.IsZero() {
		t.Fatalf("GetCart() after emptying = %+v, want no stored cart", got)
	}
}

func Test_UpdateCart_RejectsAndRollsBack(t *testing.T) {
	r := newTestRepo(t)
	user := utils.NewULID()
	variant := utils.NewULID()
	if _, err := r.UpdateCart(bg, user, addItem(variant, 1)); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	tests := []struct {
		name    string
		mutate  func(Model) (Model, error)
		wantErr error
	}{
		{name: "mutate error", mutate: func(m Model) (Model, error) { return m, boom }, wantErr: boom},
		{name: "bad variant", mutate: addItem("bad", 1), wantErr: ErrVariantIDInvalid},
		{name: "zero quantity", mutate: addItem(utils.NewULID(), 0), wantErr: ErrQuantityInvalid},
		{name: "duplicate", mutate: addItem(variant, 1), wantErr: ErrItemDuplicate},
		{name: "panic", mutate: func(Model) (Model, error) { panic("x") }, wantErr: ErrMutatePanicked},
		{name: "nil mutate", mutate: nil, wantErr: ErrMutateRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := r.UpdateCart(bg, user, tt.mutate); !errors.Is(err, tt.wantErr) {
				t.Fatalf("UpdateCart() error = %v, want %v", err, tt.wantErr)
			}
			got, _ := r.GetCart(bg, user)
			if len(got.Items) != 1 || got.Items[0].VariantID != variant || got.Items[0].Quantity != 1 {
				t.Fatalf("cart changed after failed update: %+v", got)
			}
		})
	}
}

func Test_UpdateCart_MutateGetsCopy(t *testing.T) {
	r := newTestRepo(t)
	user := utils.NewULID()
	if _, err := r.UpdateCart(bg, user, addItem(utils.NewULID(), 1)); err != nil {
		t.Fatal(err)
	}
	_, _ = r.UpdateCart(bg, user, func(m Model) (Model, error) {
		m.Items[0].Quantity = 99
		return m, errors.New("abort")
	})
	got, _ := r.GetCart(bg, user)
	if got.Items[0].Quantity != 1 {
		t.Fatalf("aborted mutate leaked change: %+v", got)
	}
}

func Test_UpdateCart_ConcurrentIsSerialized(t *testing.T) {
	r := newTestRepo(t)
	user := utils.NewULID()
	variant := utils.NewULID()
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			_, _ = r.UpdateCart(bg, user, func(m Model) (Model, error) {
				if len(m.Items) == 0 {
					m.Items = []Item{{VariantID: variant, Quantity: 1}}
					return m, nil
				}
				m.Items[0].Quantity++
				return m, nil
			})
		})
	}
	wg.Wait()
	got, _ := r.GetCart(bg, user)
	if got.Items[0].Quantity != 20 {
		t.Fatalf("quantity = %d, want 20", got.Items[0].Quantity)
	}
}

func Test_DeleteCart(t *testing.T) {
	r := newTestRepo(t)
	user := utils.NewULID()
	if _, err := r.UpdateCart(bg, user, addItem(utils.NewULID(), 1)); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteCart(bg, user); err != nil {
		t.Fatalf("DeleteCart() error = %v", err)
	}
	if err := r.DeleteCart(bg, user); err != nil {
		t.Fatalf("DeleteCart() twice error = %v", err)
	}
	got, _ := r.GetCart(bg, user)
	if len(got.Items) != 0 {
		t.Fatalf("cart not deleted: %+v", got)
	}
}

func Test_InvalidInputs(t *testing.T) {
	r := newTestRepo(t)
	ctx, cancel := context.WithCancel(bg)
	cancel()
	if _, err := r.GetCart(ctx, utils.NewULID()); !errors.Is(err, ErrCtxInvalid) {
		t.Fatalf("GetCart cancelled error = %v", err)
	}
	if _, err := r.GetCart(bg, "bad"); !errors.Is(err, ErrUserIDInvalid) {
		t.Fatalf("GetCart bad id error = %v", err)
	}
	if _, err := r.UpdateCart(bg, "bad", addItem(utils.NewULID(), 1)); !errors.Is(err, ErrUserIDInvalid) {
		t.Fatalf("UpdateCart bad id error = %v", err)
	}
	if err := r.DeleteCart(bg, "bad"); !errors.Is(err, ErrUserIDInvalid) {
		t.Fatalf("DeleteCart bad id error = %v", err)
	}
	r.now = func() (time.Time, error) { return time.Time{}, utils.ErrTimeLocationUnavailable }
	if _, err := r.UpdateCart(bg, utils.NewULID(), addItem(utils.NewULID(), 1)); !errors.Is(err, utils.ErrTimeLocationUnavailable) {
		t.Fatalf("UpdateCart clock error = %v", err)
	}
}
