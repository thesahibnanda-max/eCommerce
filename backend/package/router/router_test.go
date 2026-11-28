package router

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

type fakeHandler struct {
	panicOn string
}

func (f *fakeHandler) hit(c fiber.Ctx, name string) error {
	if name == f.panicOn {
		panic("db password leaked in panic")
	}
	if name == "AdminListProducts" && c.Query("big") == "1" {
		return c.Type("json").SendString(`{"products":"` + strings.Repeat("kurta ", 2000) + `"}`)
	}
	return c.SendString(name + "|" + c.Params("id") + c.Params("sku") + c.Params("variantID") + c.Params("category") + c.Params("slug"))
}

func (f *fakeHandler) SignUp(c fiber.Ctx) error { return f.hit(c, "SignUp") }

func (f *fakeHandler) Login(c fiber.Ctx) error { return f.hit(c, "Login") }

func (f *fakeHandler) RefreshToken(c fiber.Ctx) error { return f.hit(c, "RefreshToken") }

func (f *fakeHandler) StartEmailValidation(c fiber.Ctx) error {
	return f.hit(c, "StartEmailValidation")
}

func (f *fakeHandler) ValidateEmail(c fiber.Ctx) error { return f.hit(c, "ValidateEmail") }

func (f *fakeHandler) ListProducts(c fiber.Ctx) error { return f.hit(c, "ListProducts") }

func (f *fakeHandler) SearchProducts(c fiber.Ctx) error { return f.hit(c, "SearchProducts") }

func (f *fakeHandler) ListCategories(c fiber.Ctx) error { return f.hit(c, "ListCategories") }

func (f *fakeHandler) ListProductsByCategory(c fiber.Ctx) error {
	return f.hit(c, "ListProductsByCategory")
}

func (f *fakeHandler) GetProductBySKU(c fiber.Ctx) error { return f.hit(c, "GetProductBySKU") }

func (f *fakeHandler) GetProductByVariantID(c fiber.Ctx) error {
	return f.hit(c, "GetProductByVariantID")
}

func (f *fakeHandler) GetProductsByIDs(c fiber.Ctx) error { return f.hit(c, "GetProductsByIDs") }

func (f *fakeHandler) GetProductByID(c fiber.Ctx) error { return f.hit(c, "GetProductByID") }

func (f *fakeHandler) RecommendProducts(c fiber.Ctx) error { return f.hit(c, "RecommendProducts") }

func (f *fakeHandler) GetImage(c fiber.Ctx) error { return f.hit(c, "GetImage") }

func (f *fakeHandler) GetImageInfo(c fiber.Ctx) error { return f.hit(c, "GetImageInfo") }

func (f *fakeHandler) GetImagesInfo(c fiber.Ctx) error { return f.hit(c, "GetImagesInfo") }

func (f *fakeHandler) GetCart(c fiber.Ctx) error { return f.hit(c, "GetCart") }

func (f *fakeHandler) AddCartItem(c fiber.Ctx) error { return f.hit(c, "AddCartItem") }

func (f *fakeHandler) SetCartItemQuantity(c fiber.Ctx) error {
	return f.hit(c, "SetCartItemQuantity")
}

func (f *fakeHandler) RemoveCartItem(c fiber.Ctx) error { return f.hit(c, "RemoveCartItem") }

func (f *fakeHandler) ClearCart(c fiber.Ctx) error { return f.hit(c, "ClearCart") }

func (f *fakeHandler) MergeCart(c fiber.Ctx) error { return f.hit(c, "MergeCart") }

func (f *fakeHandler) ListInfoPages(c fiber.Ctx) error { return f.hit(c, "ListInfoPages") }

func (f *fakeHandler) GetInfoPage(c fiber.Ctx) error { return f.hit(c, "GetInfoPage") }

func (f *fakeHandler) AdminCreateProduct(c fiber.Ctx) error { return f.hit(c, "AdminCreateProduct") }

func (f *fakeHandler) AdminCreateProducts(c fiber.Ctx) error { return f.hit(c, "AdminCreateProducts") }

func (f *fakeHandler) AdminUpdateProduct(c fiber.Ctx) error { return f.hit(c, "AdminUpdateProduct") }

func (f *fakeHandler) AdminUpdateProducts(c fiber.Ctx) error { return f.hit(c, "AdminUpdateProducts") }

func (f *fakeHandler) AdminCreateImage(c fiber.Ctx) error { return f.hit(c, "AdminCreateImage") }

func (f *fakeHandler) AdminCreateImages(c fiber.Ctx) error { return f.hit(c, "AdminCreateImages") }

func (f *fakeHandler) AdminDeleteImage(c fiber.Ctx) error { return f.hit(c, "AdminDeleteImage") }

func (f *fakeHandler) AdminDeleteImages(c fiber.Ctx) error { return f.hit(c, "AdminDeleteImages") }

func (f *fakeHandler) AdminSetTracking(c fiber.Ctx) error { return f.hit(c, "AdminSetTracking") }

func (f *fakeHandler) GetMe(c fiber.Ctx) error { return f.hit(c, "GetMe") }

func (f *fakeHandler) ListAddresses(c fiber.Ctx) error { return f.hit(c, "ListAddresses") }

func (f *fakeHandler) CreateAddress(c fiber.Ctx) error { return f.hit(c, "CreateAddress") }

func (f *fakeHandler) UpdateAddress(c fiber.Ctx) error { return f.hit(c, "UpdateAddress") }

func (f *fakeHandler) DeleteAddress(c fiber.Ctx) error { return f.hit(c, "DeleteAddress") }

func (f *fakeHandler) SetDefaultAddress(c fiber.Ctx) error { return f.hit(c, "SetDefaultAddress") }

func (f *fakeHandler) GetCheckoutQuote(c fiber.Ctx) error { return f.hit(c, "GetCheckoutQuote") }

func (f *fakeHandler) PlaceOrder(c fiber.Ctx) error { return f.hit(c, "PlaceOrder") }

func (f *fakeHandler) ListOrders(c fiber.Ctx) error { return f.hit(c, "ListOrders") }

func (f *fakeHandler) GetOrder(c fiber.Ctx) error { return f.hit(c, "GetOrder") }

func (f *fakeHandler) PayOrder(c fiber.Ctx) error { return f.hit(c, "PayOrder") }

func (f *fakeHandler) SubmitPaymentProof(c fiber.Ctx) error { return f.hit(c, "SubmitPaymentProof") }

func (f *fakeHandler) CancelOrder(c fiber.Ctx) error { return f.hit(c, "CancelOrder") }

func (f *fakeHandler) PaymentWebhook(c fiber.Ctx) error { return f.hit(c, "PaymentWebhook") }

func (f *fakeHandler) PaymentRedirect(c fiber.Ctx) error { return f.hit(c, "PaymentRedirect") }

func (f *fakeHandler) AdminOverview(c fiber.Ctx) error { return f.hit(c, "AdminOverview") }
func (f *fakeHandler) AdminSnapshotStatus(c fiber.Ctx) error {
	return f.hit(c, "AdminSnapshotStatus")
}
func (f *fakeHandler) AdminRunSnapshot(c fiber.Ctx) error { return f.hit(c, "AdminRunSnapshot") }
func (f *fakeHandler) AdminConnectGoogle(c fiber.Ctx) error {
	return f.hit(c, "AdminConnectGoogle")
}
func (f *fakeHandler) AdminGoogleCallback(c fiber.Ctx) error {
	return f.hit(c, "AdminGoogleCallback")
}
func (f *fakeHandler) AdminDisconnectGoogle(c fiber.Ctx) error {
	return f.hit(c, "AdminDisconnectGoogle")
}

func (f *fakeHandler) AdminListUsers(c fiber.Ctx) error { return f.hit(c, "AdminListUsers") }

func (f *fakeHandler) AdminGetUser(c fiber.Ctx) error { return f.hit(c, "AdminGetUser") }

func (f *fakeHandler) AdminListCarts(c fiber.Ctx) error { return f.hit(c, "AdminListCarts") }

func (f *fakeHandler) AdminListProducts(c fiber.Ctx) error { return f.hit(c, "AdminListProducts") }

func (f *fakeHandler) AdminListImages(c fiber.Ctx) error { return f.hit(c, "AdminListImages") }

func (f *fakeHandler) AdminListOrders(c fiber.Ctx) error { return f.hit(c, "AdminListOrders") }

func (f *fakeHandler) AdminGetOrder(c fiber.Ctx) error { return f.hit(c, "AdminGetOrder") }

func (f *fakeHandler) AdminTransitionOrder(c fiber.Ctx) error {
	return f.hit(c, "AdminTransitionOrder")
}

func (f *fakeHandler) AdminListPayments(c fiber.Ctx) error { return f.hit(c, "AdminListPayments") }

func (f *fakeHandler) AdminGetPayment(c fiber.Ctx) error { return f.hit(c, "AdminGetPayment") }

func (f *fakeHandler) AdminGetPaymentProof(c fiber.Ctx) error {
	return f.hit(c, "AdminGetPaymentProof")
}

func (f *fakeHandler) AdminDecidePayment(c fiber.Ctx) error { return f.hit(c, "AdminDecidePayment") }

func newTestRouter(t *testing.T, panicOn string) *routerImpl {
	t.Helper()
	r, err := NewRouter(&fakeHandler{panicOn: panicOn}, testFrontendPort)
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	return r.(*routerImpl)
}

func call(t *testing.T, r *routerImpl, method, path string) (int, string) {
	t.Helper()
	resp, err := r.app.Test(httptest.NewRequest(method, path, nil))
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

const testFrontendPort = 4073

func TestNewRouter_RequiresHandler(t *testing.T) {
	if _, err := NewRouter(nil, testFrontendPort); !errors.Is(err, ErrHandlerRequired) {
		t.Fatalf("err = %v, want ErrHandlerRequired", err)
	}
	for _, port := range []uint{0, 65536} {
		if _, err := NewRouter(&fakeHandler{}, port); !errors.Is(err, ErrFrontendPortInvalid) {
			t.Fatalf("port %d err = %v, want ErrFrontendPortInvalid", port, err)
		}
	}
}

func TestCORS(t *testing.T) {
	r := newTestRouter(t, "")
	allowed := []string{"http://localhost:4073", "http://127.0.0.1:4073", "http://192.168.1.5:4073", "https://shop.example.com:4073", "http://[::1]:4073"}
	denied := []string{"http://localhost:5173", "http://evil.example:5000", "http://localhost", "ftp://localhost:4073", "null", "http://user@localhost:4073", "http://localhost:4073/path"}
	for _, origin := range allowed {
		req := httptest.NewRequest("OPTIONS", "/v1/cart/items", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
		resp, err := r.app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent || resp.Header.Get("Access-Control-Allow-Origin") != origin {
			t.Fatalf("preflight %s = %d allow-origin %q", origin, resp.StatusCode, resp.Header.Get("Access-Control-Allow-Origin"))
		}
		if !strings.Contains(strings.ToLower(resp.Header.Get("Access-Control-Allow-Headers")), "authorization") {
			t.Fatalf("preflight %s allow-headers = %q", origin, resp.Header.Get("Access-Control-Allow-Headers"))
		}
		get := httptest.NewRequest("GET", "/v1/products", nil)
		get.Header.Set("Origin", origin)
		resp, err = r.app.Test(get)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.Header.Get("Access-Control-Allow-Origin") != origin || resp.Header.Get("Access-Control-Expose-Headers") != "Retry-After" {
			t.Fatalf("simple %s headers = %v", origin, resp.Header)
		}
	}
	for _, origin := range denied {
		req := httptest.NewRequest("OPTIONS", "/v1/cart/items", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "POST")
		resp, err := r.app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("denied origin %s got allow-origin %q", origin, got)
		}
	}
}

func TestRoutes(t *testing.T) {
	r := newTestRouter(t, "")
	tests := []struct {
		method string
		path   string
		want   string
	}{
		{"POST", "/v1/users/signup", "SignUp|"},
		{"POST", "/v1/users/login", "Login|"},
		{"POST", "/v1/users/token/refresh", "RefreshToken|"},
		{"POST", "/v1/users/email/validation", "StartEmailValidation|"},
		{"POST", "/v1/users/email/validate", "ValidateEmail|"},
		{"GET", "/v1/users/me", "GetMe|"},
		{"GET", "/v1/products", "ListProducts|"},
		{"GET", "/v1/products/search", "SearchProducts|"},
		{"GET", "/v1/products/categories", "ListCategories|"},
		{"GET", "/v1/products/categories/shirts", "ListProductsByCategory|shirts"},
		{"GET", "/v1/products/sku/CS-1", "GetProductBySKU|CS-1"},
		{"GET", "/v1/products/variants/v1", "GetProductByVariantID|v1"},
		{"POST", "/v1/products/batch-get", "GetProductsByIDs|"},
		{"GET", "/v1/products/p1", "GetProductByID|p1"},
		{"GET", "/v1/products/p1/recommendations", "RecommendProducts|p1"},
		{"POST", "/v1/images/info/batch", "GetImagesInfo|"},
		{"GET", "/v1/images/i1", "GetImage|i1"},
		{"GET", "/v1/images/i1/info", "GetImageInfo|i1"},
		{"GET", "/v1/cart", "GetCart|"},
		{"DELETE", "/v1/cart", "ClearCart|"},
		{"POST", "/v1/cart/items", "AddCartItem|"},
		{"POST", "/v1/cart/merge", "MergeCart|"},
		{"GET", "/v1/info/pages", "ListInfoPages|"},
		{"GET", "/v1/info/pages/shipping-policy", "GetInfoPage|shipping-policy"},
		{"PUT", "/v1/cart/items/v1", "SetCartItemQuantity|v1"},
		{"DELETE", "/v1/cart/items/v1", "RemoveCartItem|v1"},
		{"POST", "/v1/admin/products", "AdminCreateProduct|"},
		{"POST", "/v1/admin/products/batch", "AdminCreateProducts|"},
		{"PUT", "/v1/admin/products/batch", "AdminUpdateProducts|"},
		{"PUT", "/v1/admin/products/p1", "AdminUpdateProduct|p1"},
		{"POST", "/v1/admin/images", "AdminCreateImage|"},
		{"POST", "/v1/admin/images/batch", "AdminCreateImages|"},
		{"POST", "/v1/admin/images/batch-delete", "AdminDeleteImages|"},
		{"DELETE", "/v1/admin/images/i1", "AdminDeleteImage|i1"},
		{"GET", "/v1/admin/overview", "AdminOverview|"},
		{"GET", "/v1/admin/snapshot", "AdminSnapshotStatus|"},
		{"POST", "/v1/admin/snapshot", "AdminRunSnapshot|"},
		{"POST", "/v1/admin/google/connect", "AdminConnectGoogle|"},
		{"GET", "/v1/admin/google/callback", "AdminGoogleCallback|"},
		{"DELETE", "/v1/admin/google", "AdminDisconnectGoogle|"},
		{"GET", "/v1/admin/users", "AdminListUsers|"},
		{"GET", "/v1/admin/users/u1", "AdminGetUser|u1"},
		{"GET", "/v1/admin/carts", "AdminListCarts|"},
		{"GET", "/v1/admin/products", "AdminListProducts|"},
		{"GET", "/v1/admin/images", "AdminListImages|"},
		{"GET", "/v1/admin/orders", "AdminListOrders|"},
		{"GET", "/v1/admin/orders/o1", "AdminGetOrder|o1"},
		{"POST", "/v1/admin/orders/o1/transition", "AdminTransitionOrder|o1"},
		{"POST", "/v1/admin/orders/o1/tracking", "AdminSetTracking|o1"},
		{"GET", "/v1/admin/payments", "AdminListPayments|"},
		{"GET", "/v1/admin/payments/p1", "AdminGetPayment|p1"},
		{"GET", "/v1/admin/payments/p1/proof", "AdminGetPaymentProof|p1"},
		{"POST", "/v1/admin/payments/p1/decision", "AdminDecidePayment|p1"},
		{"GET", "/v1/addresses", "ListAddresses|"},
		{"POST", "/v1/addresses", "CreateAddress|"},
		{"PUT", "/v1/addresses/a1", "UpdateAddress|a1"},
		{"DELETE", "/v1/addresses/a1", "DeleteAddress|a1"},
		{"POST", "/v1/addresses/a1/default", "SetDefaultAddress|a1"},
		{"GET", "/v1/checkout/quote", "GetCheckoutQuote|"},
		{"GET", "/v1/orders", "ListOrders|"},
		{"POST", "/v1/orders", "PlaceOrder|"},
		{"GET", "/v1/orders/o1", "GetOrder|o1"},
		{"POST", "/v1/orders/o1/pay", "PayOrder|o1"},
		{"POST", "/v1/orders/o1/payment-proof", "SubmitPaymentProof|o1"},
		{"POST", "/v1/orders/o1/cancel", "CancelOrder|o1"},
		{"POST", "/v1/payments/webhook", "PaymentWebhook|"},
		{"GET", "/v1/payments/redirect", "PaymentRedirect|"},
	}
	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			status, body := call(t, r, tc.method, tc.path)
			if status != http.StatusOK || body != tc.want {
				t.Fatalf("got %d %q, want 200 %q", status, body, tc.want)
			}
		})
	}
}

func TestUnknownRouteAndMethod(t *testing.T) {
	r := newTestRouter(t, "")
	status, body := call(t, r, "GET", "/v1/nope")
	if status != http.StatusNotFound || !strings.Contains(body, `"code":"not_found"`) {
		t.Fatalf("unknown route = %d %s", status, body)
	}
	status, body = call(t, r, "DELETE", "/v1/users/signup")
	if status != http.StatusMethodNotAllowed || !strings.Contains(body, `"code":"method_not_allowed"`) {
		t.Fatalf("wrong method = %d %s", status, body)
	}
}

func TestPanicRecovery(t *testing.T) {
	r := newTestRouter(t, "SignUp")
	status, body := call(t, r, "POST", "/v1/users/signup")
	if status != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", status)
	}
	if !strings.Contains(body, `"code":"internal_error"`) || strings.Contains(body, "password") {
		t.Fatalf("panic body = %s", body)
	}
	status, body = call(t, r, "POST", "/v1/users/login")
	if status != http.StatusOK || body != "Login|" {
		t.Fatalf("server unusable after panic: %d %s", status, body)
	}
}

func TestBodyLimit(t *testing.T) {
	r := newTestRouter(t, "")
	if r.app.Config().BodyLimit != bodyLimit {
		t.Fatalf("body limit = %d, want %d", r.app.Config().BodyLimit, bodyLimit)
	}
	big := strings.NewReader(strings.Repeat("a", 10<<20))
	resp, err := r.app.Test(httptest.NewRequest("POST", "/v1/admin/images", big), fiber.TestConfig{Timeout: 0})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("10MB body status = %d, want 200", resp.StatusCode)
	}
}

func TestListenAndServeOnPort_ServesAndShutsDown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	r := newTestRouter(t, "")
	ctx, cancel := context.WithCancel(context.Background())
	errs := r.ListenAndServeOnPort(ctx, uint(port))

	url := "http://127.0.0.1:" + strconv.Itoa(port) + "/v1/products"
	var resp *http.Response
	for range 50 {
		if resp, err = http.Get(url); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("server never came up: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b) != "ListProducts|" {
		t.Fatalf("body = %q", b)
	}

	cancel()
	select {
	case err, ok := <-errs:
		if ok && err != nil {
			t.Fatalf("listen returned error after shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down")
	}
}

func TestListenAndServeOnPort_PortInUse(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	r := newTestRouter(t, "")
	select {
	case err := <-r.ListenAndServeOnPort(context.Background(), uint(ln.Addr().(*net.TCPAddr).Port)):
		if err == nil {
			t.Fatal("expected an error for a busy port")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no error for busy port")
	}
}

func TestRouter_CompressesLargeJSON(t *testing.T) {
	r := newTestRouter(t, "")
	req := httptest.NewRequest("GET", "/v1/admin/products?big=1", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := r.app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.Header.Get("Content-Encoding") != "gzip" || len(raw) >= 12000 {
		t.Fatalf("encoding %q, %d bytes", resp.Header.Get("Content-Encoding"), len(raw))
	}
	plain := httptest.NewRequest("GET", "/v1/admin/products?big=1", nil)
	resp, err = r.app.Test(plain)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("Content-Encoding") != "" {
		t.Fatalf("clients without gzip must get plain bodies")
	}
}
