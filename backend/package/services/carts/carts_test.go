package cartsService_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/thesahibnanda-max/eCommerce/backend/package/otp"
	cartsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/carts"
	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
	usersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/users"
	cartsService "github.com/thesahibnanda-max/eCommerce/backend/package/services/carts"
	usersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/users"
	"github.com/thesahibnanda-max/eCommerce/backend/package/smtp"
	"github.com/thesahibnanda-max/eCommerce/backend/package/token"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

var phoneSeq atomic.Uint32

func nextPhone() string {
	return fmt.Sprintf("+9198765%05d", phoneSeq.Add(1))
}

const (
	testSecret   = "0123456789abcdef0123456789abcdef"
	testPassword = "Secr3t!pass"
)

var bg = context.Background()

type nopMailer struct{}

func (nopMailer) SendMail(context.Context, smtp.SendMailRequest) error { return nil }

type fixture struct {
	svc          cartsService.Service
	products     productsRepository.Repository
	carts        cartsRepository.Repository
	usersSvc     usersService.Service
	token        string
	refreshToken string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	dir := t.TempDir()
	usersRepo, err := usersRepository.NewRepository([]string{dir, "users.db"}, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = usersRepo.Close() })
	tk, err := token.NewTokenizer(testSecret)
	if err != nil {
		t.Fatal(err)
	}
	emailOTP, err := otp.NewMailOTP(nopMailer{}, otp.Config{})
	if err != nil {
		t.Fatal(err)
	}
	usersSvc, err := usersService.NewService(tk, usersRepo, emailOTP, true)
	if err != nil {
		t.Fatal(err)
	}
	user, err := usersSvc.CreateUser(bg, usersService.CreateUserRequest{Name: "A", Email: "a@example.com", PhoneNumber: nextPhone(), Password: testPassword})
	if err != nil {
		t.Fatal(err)
	}
	products, err := productsRepository.NewRepository([]string{dir, "products.db"}, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = products.Close() })
	carts, err := cartsRepository.NewRepository([]string{dir, "carts.db"}, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = carts.Close() })
	svc, err := cartsService.NewService(usersSvc, products, carts)
	if err != nil {
		t.Fatal(err)
	}
	return fixture{svc: svc, products: products, carts: carts, usersSvc: usersSvc, token: user.AccessToken, refreshToken: user.RefreshToken}
}

func (f fixture) seed(t *testing.T, sku string, price int64, currency string, stock uint64) productsRepository.Variant {
	t.Helper()
	p, err := f.products.CreateProduct(bg, productsRepository.Model{
		Name:     "Product " + sku,
		Variants: []productsRepository.Variant{{SKU: sku, ShownPrice: price + 100, SellingPrice: price, Currency: currency, StockUnits: stock, Attributes: map[string]string{"size": "M"}, ImageURLs: []string{"/v1/images/" + sku}}},
	})
	if err != nil {
		t.Fatalf("CreateProduct(%s): %v", sku, err)
	}
	return p.Variants[0]
}

func (f fixture) newUserToken(t *testing.T, email string) string {
	t.Helper()
	user, err := f.usersSvc.CreateUser(bg, usersService.CreateUserRequest{Name: "B", Email: email, PhoneNumber: nextPhone(), Password: testPassword})
	if err != nil {
		t.Fatal(err)
	}
	return user.AccessToken
}

func (f fixture) add(t *testing.T, variantID string, quantity int) cartsService.Cart {
	t.Helper()
	res, err := f.svc.AddItem(bg, cartsService.AddItemRequest{AccessToken: f.token, VariantID: variantID, Quantity: quantity})
	if err != nil {
		t.Fatalf("AddItem(%s, %d): %v", variantID, quantity, err)
	}
	return res.Cart
}

func Test_NewService(t *testing.T) {
	f := newFixture(t)
	if _, err := cartsService.NewService(nil, f.products, f.carts); !errors.Is(err, cartsService.ErrUsersServiceRequired) {
		t.Fatalf("nil users err = %v", err)
	}
	if _, err := cartsService.NewService(f.usersSvc, nil, f.carts); !errors.Is(err, cartsService.ErrProductsRepoRequired) {
		t.Fatalf("nil products err = %v", err)
	}
	if _, err := cartsService.NewService(f.usersSvc, f.products, nil); !errors.Is(err, cartsService.ErrCartsRepoRequired) {
		t.Fatalf("nil carts err = %v", err)
	}
}

func Test_EveryMethodRequiresAccessToken(t *testing.T) {
	f := newFixture(t)
	v := f.seed(t, "A-1", 100, "INR", 5)
	for _, tk := range []struct {
		name    string
		token   string
		wantErr error
	}{
		{"empty", "", cartsService.ErrTokenInvalid},
		{"garbage", "x.y.z", cartsService.ErrTokenInvalid},
		{"refresh", f.refreshToken, cartsService.ErrTokenWrongType},
	} {
		calls := map[string]func() error{
			"GetCart": func() error {
				_, err := f.svc.GetCart(bg, cartsService.GetCartRequest{AccessToken: tk.token})
				return err
			},
			"AddItem": func() error {
				_, err := f.svc.AddItem(bg, cartsService.AddItemRequest{AccessToken: tk.token, VariantID: v.ID, Quantity: 1})
				return err
			},
			"SetItemQuantity": func() error {
				_, err := f.svc.SetItemQuantity(bg, cartsService.SetItemQuantityRequest{AccessToken: tk.token, VariantID: v.ID, Quantity: 1})
				return err
			},
			"RemoveItem": func() error {
				_, err := f.svc.RemoveItem(bg, cartsService.RemoveItemRequest{AccessToken: tk.token, VariantID: v.ID})
				return err
			},
			"ClearCart": func() error {
				_, err := f.svc.ClearCart(bg, cartsService.ClearCartRequest{AccessToken: tk.token})
				return err
			},
			"MergeItems": func() error {
				_, err := f.svc.MergeItems(bg, cartsService.MergeItemsRequest{AccessToken: tk.token, Items: []cartsService.ItemInput{{VariantID: v.ID, Quantity: 1}}})
				return err
			},
		}
		for name, call := range calls {
			if err := call(); !errors.Is(err, tk.wantErr) {
				t.Errorf("%s with %s token: err = %v, want %v", name, tk.name, err, tk.wantErr)
			}
		}
	}
}

func Test_AddItem_EnrichesAndTotals(t *testing.T) {
	f := newFixture(t)
	a := f.seed(t, "A-1", 100, "INR", 5)
	b := f.seed(t, "B-1", 30, "USD", 5)
	f.add(t, a.ID, 2)
	f.add(t, a.ID, 1)
	cart := f.add(t, b.ID, 1)
	if len(cart.Items) != 2 || cart.ItemCount != 4 {
		t.Fatalf("cart = %+v", cart)
	}
	line := cart.Items[0]
	if line.VariantID != a.ID || line.Quantity != 3 || line.SKU != "A-1" || line.Name != "Product A-1" || line.ImageURL != "/v1/images/A-1" || !line.Available || line.Attributes["size"] != "M" {
		t.Fatalf("line = %+v", line)
	}
	if len(cart.Totals) != 2 || cart.Totals[0].Currency != "INR" || cart.Totals[0].SellingPrice != 300 || cart.Totals[0].ShownPrice != 600 || cart.Totals[1].Currency != "USD" || cart.Totals[1].SellingPrice != 30 {
		t.Fatalf("totals = %+v", cart.Totals)
	}
	got, err := f.svc.GetCart(bg, cartsService.GetCartRequest{AccessToken: f.token})
	if err != nil || got.Cart.ItemCount != 4 {
		t.Fatalf("GetCart() = %+v, %v", got, err)
	}
}

func Test_AddItem_Errors(t *testing.T) {
	f := newFixture(t)
	v := f.seed(t, "A-1", 100, "INR", 3)
	f.add(t, v.ID, 2)
	tests := []struct {
		name     string
		variant  string
		quantity int
		wantErr  error
	}{
		{"zero", v.ID, 0, cartsService.ErrQuantityInvalid},
		{"too large", v.ID, 11, cartsService.ErrQuantityTooLarge},
		{"over stock", v.ID, 2, cartsService.ErrOutOfStock},
		{"bad id", "bad", 1, cartsService.ErrVariantIDInvalid},
		{"missing", utils.NewULID(), 1, cartsService.ErrVariantNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := f.svc.AddItem(bg, cartsService.AddItemRequest{AccessToken: f.token, VariantID: tt.variant, Quantity: tt.quantity}); !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
	got, _ := f.svc.GetCart(bg, cartsService.GetCartRequest{AccessToken: f.token})
	if got.Cart.ItemCount != 2 {
		t.Fatalf("failed adds changed cart: %+v", got.Cart)
	}
}

func Test_AddItem_CartFull(t *testing.T) {
	f := newFixture(t)
	for i := range 50 {
		v := f.seed(t, "S-"+string(rune('A'+i/26))+string(rune('a'+i%26)), 10, "INR", 5)
		f.add(t, v.ID, 1)
	}
	extra := f.seed(t, "EXTRA", 10, "INR", 5)
	if _, err := f.svc.AddItem(bg, cartsService.AddItemRequest{AccessToken: f.token, VariantID: extra.ID, Quantity: 1}); !errors.Is(err, cartsService.ErrCartFull) {
		t.Fatalf("err = %v, want ErrCartFull", err)
	}
}

func Test_SetItemQuantity(t *testing.T) {
	f := newFixture(t)
	v := f.seed(t, "A-1", 100, "INR", 4)
	f.add(t, v.ID, 1)
	res, err := f.svc.SetItemQuantity(bg, cartsService.SetItemQuantityRequest{AccessToken: f.token, VariantID: v.ID, Quantity: 4})
	if err != nil || res.Cart.Items[0].Quantity != 4 {
		t.Fatalf("set 4 = %+v, %v", res, err)
	}
	if _, err := f.svc.SetItemQuantity(bg, cartsService.SetItemQuantityRequest{AccessToken: f.token, VariantID: v.ID, Quantity: 5}); !errors.Is(err, cartsService.ErrOutOfStock) {
		t.Fatalf("set 5 err = %v", err)
	}
	if _, err := f.svc.SetItemQuantity(bg, cartsService.SetItemQuantityRequest{AccessToken: f.token, VariantID: v.ID, Quantity: -1}); !errors.Is(err, cartsService.ErrQuantityInvalid) {
		t.Fatalf("set -1 err = %v", err)
	}
	if _, err := f.svc.SetItemQuantity(bg, cartsService.SetItemQuantityRequest{AccessToken: f.token, VariantID: utils.NewULID(), Quantity: 1}); !errors.Is(err, cartsService.ErrCartItemNotFound) {
		t.Fatalf("set missing err = %v", err)
	}
	res, err = f.svc.SetItemQuantity(bg, cartsService.SetItemQuantityRequest{AccessToken: f.token, VariantID: v.ID, Quantity: 0})
	if err != nil || len(res.Cart.Items) != 0 {
		t.Fatalf("set 0 = %+v, %v", res, err)
	}
}

func Test_RemoveAndClear(t *testing.T) {
	f := newFixture(t)
	a := f.seed(t, "A-1", 100, "INR", 4)
	b := f.seed(t, "B-1", 100, "INR", 4)
	f.add(t, a.ID, 1)
	f.add(t, b.ID, 1)
	res, err := f.svc.RemoveItem(bg, cartsService.RemoveItemRequest{AccessToken: f.token, VariantID: a.ID})
	if err != nil || len(res.Cart.Items) != 1 || res.Cart.Items[0].VariantID != b.ID {
		t.Fatalf("RemoveItem() = %+v, %v", res, err)
	}
	if _, err := f.svc.RemoveItem(bg, cartsService.RemoveItemRequest{AccessToken: f.token, VariantID: a.ID}); !errors.Is(err, cartsService.ErrCartItemNotFound) {
		t.Fatalf("RemoveItem twice err = %v", err)
	}
	cleared, err := f.svc.ClearCart(bg, cartsService.ClearCartRequest{AccessToken: f.token})
	if err != nil || len(cleared.Cart.Items) != 0 || cleared.Cart.Items == nil || cleared.Cart.Totals == nil {
		t.Fatalf("ClearCart() = %+v, %v", cleared, err)
	}
	got, _ := f.svc.GetCart(bg, cartsService.GetCartRequest{AccessToken: f.token})
	if len(got.Cart.Items) != 0 {
		t.Fatalf("cart after clear = %+v", got.Cart)
	}
}

func Test_MergeItems(t *testing.T) {
	f := newFixture(t)
	a := f.seed(t, "A-1", 100, "INR", 4)
	b := f.seed(t, "B-1", 50, "INR", 20)
	empty := f.seed(t, "E-1", 50, "INR", 0)
	missing := utils.NewULID()
	f.add(t, a.ID, 2)
	res, err := f.svc.MergeItems(bg, cartsService.MergeItemsRequest{AccessToken: f.token, Items: []cartsService.ItemInput{
		{VariantID: a.ID, Quantity: 5},
		{VariantID: b.ID, Quantity: 8},
		{VariantID: " " + b.ID + " ", Quantity: 8},
		{VariantID: empty.ID, Quantity: 1},
		{VariantID: missing, Quantity: 1},
	}})
	if err != nil {
		t.Fatalf("MergeItems() error = %v", err)
	}
	quantities := map[string]int{}
	for _, item := range res.Cart.Items {
		quantities[item.VariantID] = item.Quantity
	}
	if quantities[a.ID] != 4 || quantities[b.ID] != 10 || len(quantities) != 2 {
		t.Fatalf("quantities = %v, want a capped at stock 4 and b capped at 10", quantities)
	}
	if len(res.Skipped) != 2 || res.Skipped[0] != empty.ID || res.Skipped[1] != missing {
		t.Fatalf("skipped = %v", res.Skipped)
	}
}

func Test_MergeItems_ValidationAndAllOrNothing(t *testing.T) {
	f := newFixture(t)
	a := f.seed(t, "A-1", 100, "INR", 4)
	f.add(t, a.ID, 1)
	tests := []struct {
		name    string
		items   []cartsService.ItemInput
		wantErr error
	}{
		{"empty", nil, cartsService.ErrBatchEmpty},
		{"bad id", []cartsService.ItemInput{{VariantID: a.ID, Quantity: 1}, {VariantID: "bad", Quantity: 1}}, cartsService.ErrVariantIDInvalid},
		{"zero quantity", []cartsService.ItemInput{{VariantID: a.ID, Quantity: 0}}, cartsService.ErrQuantityInvalid},
		{"too many", make([]cartsService.ItemInput, 51), cartsService.ErrTooManyItems},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := f.svc.MergeItems(bg, cartsService.MergeItemsRequest{AccessToken: f.token, Items: tt.items}); !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
	var many []cartsService.ItemInput
	for i := range 50 {
		v := f.seed(t, "M-"+string(rune('A'+i/26))+string(rune('a'+i%26)), 10, "INR", 5)
		many = append(many, cartsService.ItemInput{VariantID: v.ID, Quantity: 1})
	}
	if _, err := f.svc.MergeItems(bg, cartsService.MergeItemsRequest{AccessToken: f.token, Items: many}); !errors.Is(err, cartsService.ErrCartFull) {
		t.Fatalf("overflow err = %v, want ErrCartFull", err)
	}
	got, _ := f.svc.GetCart(bg, cartsService.GetCartRequest{AccessToken: f.token})
	if len(got.Cart.Items) != 1 || got.Cart.Items[0].Quantity != 1 {
		t.Fatalf("cart changed after failed merge: %+v", got.Cart)
	}
}

func Test_UnavailableLines(t *testing.T) {
	f := newFixture(t)
	v := f.seed(t, "A-1", 100, "INR", 5)
	f.add(t, v.ID, 3)
	product, _, err := f.products.GetProductByVariantID(bg, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	product.Variants[0].StockUnits = 1
	if _, err := f.products.UpdateProduct(bg, product); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.GetCart(bg, cartsService.GetCartRequest{AccessToken: f.token})
	if err != nil {
		t.Fatal(err)
	}
	if got.Cart.Items[0].Available || len(got.Cart.Totals) != 0 {
		t.Fatalf("cart = %+v, want line unavailable and excluded from totals", got.Cart)
	}
}

func Test_CartsArePerUser(t *testing.T) {
	f := newFixture(t)
	v := f.seed(t, "A-1", 100, "INR", 5)
	f.add(t, v.ID, 2)
	other := f.newUserToken(t, "b@example.com")
	got, err := f.svc.GetCart(bg, cartsService.GetCartRequest{AccessToken: other})
	if err != nil || len(got.Cart.Items) != 0 {
		t.Fatalf("other user cart = %+v, %v", got, err)
	}
}

func Test_ConcurrentAdds(t *testing.T) {
	f := newFixture(t)
	v := f.seed(t, "A-1", 100, "INR", 100)
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			_, _ = f.svc.AddItem(bg, cartsService.AddItemRequest{AccessToken: f.token, VariantID: v.ID, Quantity: 1})
		})
	}
	wg.Wait()
	got, _ := f.svc.GetCart(bg, cartsService.GetCartRequest{AccessToken: f.token})
	if got.Cart.Items[0].Quantity != 10 {
		t.Fatalf("quantity = %d, want 10", got.Cart.Items[0].Quantity)
	}
}

func Test_CancelledContext(t *testing.T) {
	f := newFixture(t)
	ctx, cancel := context.WithCancel(bg)
	cancel()
	if _, err := f.svc.GetCart(ctx, cartsService.GetCartRequest{AccessToken: f.token}); !errors.Is(err, cartsService.ErrInvalidContext) {
		t.Fatalf("err = %v", err)
	}
}
