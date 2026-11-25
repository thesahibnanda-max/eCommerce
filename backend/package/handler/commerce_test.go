package handler_test

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"

	addressesService "github.com/thesahibnanda-max/eCommerce/backend/package/services/addresses"
	adminService "github.com/thesahibnanda-max/eCommerce/backend/package/services/admin"
	adminOperationsService "github.com/thesahibnanda-max/eCommerce/backend/package/services/admin_operations"
	cartsService "github.com/thesahibnanda-max/eCommerce/backend/package/services/carts"
	ordersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/orders"
	usersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/users"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

type shopper struct {
	token     string
	userID    string
	addressID string
}

func (f fixture) createStockedProduct(t *testing.T, sku string, price int64, stock uint64) string {
	t.Helper()
	body := `{"name":"Kurta ` + sku + `","description":"cotton","variants":[{"sku":"` + sku + `","shown_price":` + itoa(price+100) + `,"selling_price":` + itoa(price) + `,"currency":"INR","stock_units":` + itoa(int64(stock)) + `}]}`
	r := f.do(t, request{method: "POST", path: "/v1/admin/products", body: body, adminKey: testAdminKey})
	expectStatus(t, r, http.StatusCreated)
	return decode[adminService.CreateProductResponse](t, r).Product.Variants[0].ID
}

func (f fixture) shopper(t *testing.T, email string) shopper {
	t.Helper()
	created := f.signUp(t, email)
	r := f.do(t, request{method: "POST", path: "/v1/addresses", bearer: created.AccessToken, body: `{"label":"Home","full_name":"Asha Rao","phone_number":"9812345678","address_line1":"12 MG Road","pincode":"560001"}`})
	expectStatus(t, r, http.StatusCreated)
	address := decode[addressesService.AddressResponse](t, r).Address
	if address.City == "" || address.State == "" || !address.IsDefault || address.Country != "India" {
		t.Fatalf("address not filled from pincode: %+v", address)
	}
	return shopper{token: created.AccessToken, userID: created.ID, addressID: address.ID}
}

func (f fixture) addToCart(t *testing.T, token, variantID string, quantity int) {
	t.Helper()
	r := f.do(t, request{method: "POST", path: "/v1/cart/items", bearer: token, body: `{"variant_id":"` + variantID + `","quantity":` + itoa(int64(quantity)) + `}`})
	expectStatus(t, r, http.StatusOK)
}

func (f fixture) placeOrder(t *testing.T, s shopper, key string) ordersService.Order {
	t.Helper()
	r := f.do(t, request{method: "POST", path: "/v1/orders", bearer: s.token, body: `{"address_id":"` + s.addressID + `","idempotency_key":"` + key + `"}`})
	expectStatus(t, r, http.StatusCreated)
	return decode[ordersService.OrderResponse](t, r).Order
}

func (f fixture) getOrder(t *testing.T, token, id string) ordersService.Order {
	t.Helper()
	r := f.do(t, request{method: "GET", path: "/v1/orders/" + id, bearer: token})
	expectStatus(t, r, http.StatusOK)
	return decode[ordersService.OrderResponse](t, r).Order
}

func (f fixture) submitProof(t *testing.T, token, orderID, txn string) response {
	t.Helper()
	body, contentType := multipartBody(t, "file", map[string][]byte{"proof.png": pngBytes(t, 8)}, map[string]string{"transaction_id": txn})
	return f.do(t, request{method: "POST", path: "/v1/orders/" + orderID + "/payment-proof", bearer: token, raw: body, contentType: contentType})
}

func (f fixture) stock(t *testing.T, variantID string) uint64 {
	t.Helper()
	_, variant, err := f.products.GetProductByVariantID(context.Background(), variantID)
	if err != nil {
		t.Fatal(err)
	}
	return variant.StockUnits
}

func (f fixture) mailsTo(email string) []string {
	f.mailer.mu.Lock()
	defer f.mailer.mu.Unlock()
	var subjects []string
	for _, mail := range f.mailer.sent {
		if mail.To == email {
			subjects = append(subjects, mail.Subject)
		}
	}
	return subjects
}

func hasSubject(subjects []string, fragment string) bool {
	for _, subject := range subjects {
		if strings.Contains(subject, fragment) {
			return true
		}
	}
	return false
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}

func TestCheckout_UPIApprovedEndToEnd(t *testing.T) {
	f := newFixture(t)
	variant := f.createStockedProduct(t, "KURTA-RED", 600, 3)
	s := f.shopper(t, "buyer@example.com")
	f.addToCart(t, s.token, variant, 2)

	catalog := f.do(t, request{method: "GET", path: "/v1/products"})
	if !strings.Contains(catalog.headers.Get("Cache-Control"), "max-age=60") {
		t.Fatalf("catalog must be cacheable, got %q", catalog.headers.Get("Cache-Control"))
	}
	if f.do(t, request{method: "GET", path: "/v1/orders", bearer: s.token}).headers.Get("Cache-Control") != "" {
		t.Fatalf("private endpoints must not be cached")
	}

	r := f.do(t, request{method: "GET", path: "/v1/checkout/quote", bearer: s.token})
	expectStatus(t, r, http.StatusOK)
	quote := decode[ordersService.QuoteResponse](t, r).Quote
	if !quote.Ready || quote.Subtotal != 1200 || quote.ShippingFee != 0 || quote.TotalPayable != 1200 || quote.PaymentProvider != "SELF_UPI" || quote.ItemCount != 2 {
		t.Fatalf("quote %+v", quote)
	}

	key := utils.NewULID()
	order := f.placeOrder(t, s, key)
	if order.State != "CREATED" || order.TotalPayable != 1200 || order.Payment == nil || order.Payment.State != "AWAITING_PAYMENT" || !order.CanCancel || !order.CanSubmitProof || order.ShippingAddress.Pincode != "560001" {
		t.Fatalf("order %+v", order)
	}
	action := order.Payment.Action
	if action == nil || action.Type != "SHOW_QR" || action.UPIID == "" || !strings.HasPrefix(action.QRImageDataURI, "data:image/") || !strings.Contains(action.UPIIntentURL, "am=1200.00") {
		t.Fatalf("action %+v", action)
	}
	if got := f.stock(t, variant); got != 1 {
		t.Fatalf("stock after order = %d, want 1", got)
	}
	cart := f.do(t, request{method: "GET", path: "/v1/cart", bearer: s.token})
	if len(decode[cartsService.GetCartResponse](t, cart).Cart.Items) != 0 {
		t.Fatalf("cart must be cleared after order")
	}
	again := f.placeOrder(t, s, key)
	if again.ID != order.ID || f.stock(t, variant) != 1 {
		t.Fatalf("same checkout key must return the same order without reserving again")
	}
	if reloaded := f.getOrder(t, s.token, order.ID); reloaded.Payment.Action == nil || reloaded.Payment.Action.UPIID != action.UPIID {
		t.Fatalf("payment instructions must be shown again on the order page")
	}

	expectError(t, f.submitProof(t, s.token, order.ID, "12"), http.StatusBadRequest, "transaction_id_invalid")
	r = f.submitProof(t, s.token, order.ID, "412345678901")
	expectStatus(t, r, http.StatusOK)
	order = decode[ordersService.OrderResponse](t, r).Order
	if order.State != "PROCESSING" || order.Payment.State != "VERIFYING" || order.Payment.ProofTransactionID != "412345678901" || order.CanCancel {
		t.Fatalf("after proof %+v", order)
	}
	expectError(t, f.do(t, request{method: "POST", path: "/v1/orders/" + order.ID + "/cancel", bearer: s.token}), http.StatusConflict, "order_cannot_cancel")

	r = f.do(t, request{method: "GET", path: "/v1/admin/overview", adminKey: testAdminKey})
	expectStatus(t, r, http.StatusOK)
	overview := decode[adminOperationsService.OverviewResponse](t, r).Overview
	if overview.PaymentsAwaitingReview != 1 || overview.Users != 1 || overview.Products != 1 || overview.OrdersByState["PROCESSING"] != 1 || overview.PaymentProvider != "SELF_UPI" {
		t.Fatalf("overview %+v", overview)
	}
	r = f.do(t, request{method: "GET", path: "/v1/admin/payments?states=VERIFYING", adminKey: testAdminKey})
	expectStatus(t, r, http.StatusOK)
	payments := decode[ordersService.ListPaymentsResponse](t, r).Payments
	if len(payments) != 1 || payments[0].ID != order.Payment.ID || !payments[0].HasProofImage {
		t.Fatalf("payments %+v", payments)
	}
	proof := f.do(t, request{method: "GET", path: "/v1/admin/payments/" + order.Payment.ID + "/proof", adminKey: testAdminKey})
	expectStatus(t, proof, http.StatusOK)
	if !bytes.HasPrefix(proof.body, []byte("\x89PNG")) || proof.headers.Get("Content-Type") != "image/png" {
		t.Fatalf("proof image not served")
	}
	expectError(t, f.do(t, request{method: "POST", path: "/v1/admin/payments/" + order.Payment.ID + "/decision", adminKey: "wrong", body: `{"status":"SUCCEEDED"}`}), http.StatusUnauthorized, "admin_unauthorized")
	expectError(t, f.do(t, request{method: "POST", path: "/v1/admin/payments/" + order.Payment.ID + "/decision", adminKey: testAdminKey, body: `{"status":"MAYBE"}`}), http.StatusBadRequest, "decision_invalid")
	r = f.do(t, request{method: "POST", path: "/v1/admin/payments/" + order.Payment.ID + "/decision", adminKey: testAdminKey, body: `{"status":"SUCCEEDED","note":"seen in bank app"}`})
	expectStatus(t, r, http.StatusOK)
	if decode[ordersService.PaymentResponse](t, r).Payment.State != "SUCCEEDED" {
		t.Fatalf("decision not applied")
	}
	f.orders.Wait()

	order = f.getOrder(t, s.token, order.ID)
	if order.State != "APPROVED" || order.Payment.State != "SUCCEEDED" || order.CanCancel || order.CanPay || len(order.History) < 3 {
		t.Fatalf("after approval %+v", order)
	}
	r = f.do(t, request{method: "GET", path: "/v1/orders", bearer: s.token})
	expectStatus(t, r, http.StatusOK)
	list := decode[ordersService.ListOrdersResponse](t, r).Orders
	if len(list) != 1 || list[0].State != "APPROVED" || list[0].Payment == nil || list[0].Payment.State != "SUCCEEDED" || list[0].Metadata != nil || list[0].UserID != "" {
		t.Fatalf("order list %+v", list)
	}
	if !hasSubject(f.mailsTo("buyer@example.com"), "Payment received") {
		t.Fatalf("buyer did not get the payment received email: %v", f.mailsTo("buyer@example.com"))
	}

	expectError(t, f.do(t, request{method: "POST", path: "/v1/admin/orders/" + order.ID + "/transition", adminKey: testAdminKey, body: `{"to":"DELIVERED"}`}), http.StatusConflict, "transition_not_allowed")
	r = f.do(t, request{method: "POST", path: "/v1/admin/orders/" + order.ID + "/transition", adminKey: testAdminKey, body: `{"to":"STARTED_SHIPPING","reason":"handed to courier"}`})
	expectStatus(t, r, http.StatusOK)
	adminOrder := decode[ordersService.OrderResponse](t, r).Order
	if adminOrder.State != "STARTED_SHIPPING" || adminOrder.UserID != s.userID || len(adminOrder.Payments) != 1 || len(adminOrder.AllowedTransitions) != 3 {
		t.Fatalf("admin order %+v", adminOrder)
	}
	f.orders.Wait()
	if !hasSubject(f.mailsTo("buyer@example.com"), "has shipped") {
		t.Fatalf("buyer did not get the shipped email")
	}
	if f.stock(t, variant) != 1 {
		t.Fatalf("shipping must not touch stock")
	}

	r = f.do(t, request{method: "GET", path: "/v1/admin/users/" + s.userID, adminKey: testAdminKey})
	expectStatus(t, r, http.StatusOK)
	detail := decode[adminOperationsService.UserDetailResponse](t, r)
	if detail.User.Email != "buyer@example.com" || len(detail.Addresses) != 1 || len(detail.Orders) != 1 || len(detail.Cart.Lines) != 0 {
		t.Fatalf("user detail %+v", detail)
	}
	for _, path := range []string{"/v1/admin/users", "/v1/admin/carts", "/v1/admin/products", "/v1/admin/images", "/v1/admin/orders?states=STARTED_SHIPPING", "/v1/admin/orders/" + order.ID, "/v1/admin/payments/" + order.Payment.ID} {
		expectStatus(t, f.do(t, request{method: "GET", path: path, adminKey: testAdminKey}), http.StatusOK)
		expectError(t, f.do(t, request{method: "GET", path: path}), http.StatusUnauthorized, "unauthorized")
	}
}

func TestCheckout_RejectedPaymentFailsOrderAndRestoresStock(t *testing.T) {
	f := newFixture(t)
	variant := f.createStockedProduct(t, "SAREE-1", 400, 5)
	s := f.shopper(t, "rejected@example.com")
	f.addToCart(t, s.token, variant, 2)
	order := f.placeOrder(t, s, utils.NewULID())
	if order.ShippingFee != 79 || order.TotalPayable != 879 {
		t.Fatalf("shipping fee below free threshold: %+v", order)
	}
	if f.stock(t, variant) != 3 {
		t.Fatalf("stock not reserved")
	}
	expectStatus(t, f.submitProof(t, s.token, order.ID, "UTR998877"), http.StatusOK)
	r := f.do(t, request{method: "POST", path: "/v1/admin/payments/" + order.Payment.ID + "/decision", adminKey: testAdminKey, body: `{"status":"FAILED","note":"no money received"}`})
	expectStatus(t, r, http.StatusOK)
	f.orders.Wait()
	order = f.getOrder(t, s.token, order.ID)
	if order.State != "FAILED" || order.Payment.State != "FAILED" || order.Payment.FailureReason != "no money received" {
		t.Fatalf("after rejection %+v", order)
	}
	if f.stock(t, variant) != 5 {
		t.Fatalf("stock not restored after rejection: %d", f.stock(t, variant))
	}
	if !hasSubject(f.mailsTo("rejected@example.com"), "Payment not confirmed") {
		t.Fatalf("buyer did not get the rejection email")
	}
	r = f.do(t, request{method: "POST", path: "/v1/admin/payments/" + order.Payment.ID + "/decision", adminKey: testAdminKey, body: `{"status":"SUCCEEDED"}`})
	expectStatus(t, r, http.StatusOK)
	f.orders.Wait()
	r = f.do(t, request{method: "GET", path: "/v1/admin/orders/" + order.ID, adminKey: testAdminKey})
	adminOrder := decode[ordersService.OrderResponse](t, r).Order
	if adminOrder.State != "FAILED" || adminOrder.Metadata["needs_refund"] != true {
		t.Fatalf("late approval of a failed order must flag a refund: %+v", adminOrder)
	}
}

func TestCheckout_CustomerCancelRestoresStock(t *testing.T) {
	f := newFixture(t)
	variant := f.createStockedProduct(t, "DUPATTA-1", 300, 2)
	s := f.shopper(t, "cancel@example.com")
	f.addToCart(t, s.token, variant, 2)
	order := f.placeOrder(t, s, utils.NewULID())
	if f.stock(t, variant) != 0 {
		t.Fatalf("stock not reserved")
	}
	r := f.do(t, request{method: "POST", path: "/v1/orders/" + order.ID + "/cancel", bearer: s.token})
	expectStatus(t, r, http.StatusOK)
	order = decode[ordersService.OrderResponse](t, r).Order
	if order.State != "CANCELLED" || order.Payment.State != "FAILED" || order.CanCancel || order.CanPay || order.CanSubmitProof {
		t.Fatalf("after cancel %+v", order)
	}
	if f.stock(t, variant) != 2 {
		t.Fatalf("stock not restored after cancel")
	}
	expectError(t, f.do(t, request{method: "POST", path: "/v1/orders/" + order.ID + "/cancel", bearer: s.token}), http.StatusConflict, "order_cannot_cancel")
	expectError(t, f.submitProof(t, s.token, order.ID, "412345678901"), http.StatusConflict, "proof_not_allowed")
	expectError(t, f.do(t, request{method: "POST", path: "/v1/orders/" + order.ID + "/pay", bearer: s.token}), http.StatusConflict, "order_cannot_pay")
}

func TestCheckout_GuardsAndOwnership(t *testing.T) {
	f := newFixture(t)
	variant := f.createStockedProduct(t, "TOP-1", 500, 2)
	a := f.shopper(t, "a-shopper@example.com")
	b := f.shopper(t, "b-shopper@example.com")

	expectError(t, f.do(t, request{method: "POST", path: "/v1/orders", bearer: a.token, body: `{"address_id":"` + a.addressID + `","idempotency_key":"k1"}`}), http.StatusBadRequest, "cart_empty")
	f.addToCart(t, a.token, variant, 2)
	expectError(t, f.do(t, request{method: "POST", path: "/v1/orders", bearer: a.token, body: `{"address_id":"","idempotency_key":"k1"}`}), http.StatusBadRequest, "address_required")
	expectError(t, f.do(t, request{method: "POST", path: "/v1/orders", bearer: a.token, body: `{"address_id":"` + b.addressID + `","idempotency_key":"k1"}`}), http.StatusNotFound, "address_not_found")
	expectError(t, f.do(t, request{method: "POST", path: "/v1/orders", bearer: a.token, body: `{"address_id":"` + a.addressID + `","idempotency_key":""}`}), http.StatusBadRequest, "idempotency_key_invalid")

	f.addToCart(t, b.token, variant, 2)
	f.placeOrder(t, b, utils.NewULID())
	r := f.do(t, request{method: "GET", path: "/v1/checkout/quote", bearer: a.token})
	if quote := decode[ordersService.QuoteResponse](t, r).Quote; quote.Ready || len(quote.Problems) == 0 {
		t.Fatalf("quote must report the stock problem: %+v", quote)
	}
	expectError(t, f.do(t, request{method: "POST", path: "/v1/orders", bearer: a.token, body: `{"address_id":"` + a.addressID + `","idempotency_key":"k2"}`}), http.StatusConflict, "out_of_stock")

	r = f.do(t, request{method: "GET", path: "/v1/orders", bearer: b.token})
	bOrder := decode[ordersService.ListOrdersResponse](t, r).Orders[0]
	expectError(t, f.do(t, request{method: "GET", path: "/v1/orders/" + bOrder.ID, bearer: a.token}), http.StatusNotFound, "order_not_found")
	expectError(t, f.submitProof(t, a.token, bOrder.ID, "412345678901"), http.StatusNotFound, "order_not_found")
	expectError(t, f.do(t, request{method: "GET", path: "/v1/orders"}), http.StatusUnauthorized, "unauthorized")

	expectError(t, f.do(t, request{method: "POST", path: "/v1/payments/webhook", body: "payment_id=x&status=SUCCEEDED", contentType: "application/x-www-form-urlencoded"}), http.StatusNotFound, "not_supported")
	expectError(t, f.do(t, request{method: "GET", path: "/v1/payments/redirect?payment_request_id=x"}), http.StatusNotFound, "not_supported")

	r = f.do(t, request{method: "PUT", path: "/v1/addresses/" + a.addressID, bearer: a.token, body: `{"label":"Work","full_name":"Asha Rao","phone_number":"9812345678","address_line1":"1 Residency Road","pincode":"110001","is_default":true}`})
	expectStatus(t, r, http.StatusOK)
	if updated := decode[addressesService.AddressResponse](t, r).Address; updated.Label != "Work" || updated.Pincode != "110001" || updated.State == "Karnataka" {
		t.Fatalf("address update %+v", updated)
	}
	expectError(t, f.do(t, request{method: "POST", path: "/v1/addresses", bearer: a.token, body: `{"full_name":"A","phone_number":"9812345678","address_line1":"x","pincode":"999999"}`}), http.StatusBadRequest, "pincode_not_found")
	expectStatus(t, f.do(t, request{method: "DELETE", path: "/v1/addresses/" + a.addressID, bearer: a.token}), http.StatusOK)
	r = f.do(t, request{method: "GET", path: "/v1/addresses", bearer: a.token})
	if len(decode[addressesService.ListAddressesResponse](t, r).Addresses) != 0 {
		t.Fatalf("address not deleted")
	}
}

func TestAdmin_DataViewsAndOrderActions(t *testing.T) {
	f := newFixture(t)
	variant := f.createStockedProduct(t, "LEHENGA-1", 1500, 4)
	s := f.shopper(t, "views@example.com")

	second := f.do(t, request{method: "POST", path: "/v1/addresses", bearer: s.token, body: `{"label":"Office","full_name":"Asha Rao","phone_number":"9812345678","address_line1":"2 Brigade Road","pincode":"560001"}`})
	expectStatus(t, second, http.StatusCreated)
	secondID := decode[addressesService.AddressResponse](t, second).Address.ID
	r := f.do(t, request{method: "POST", path: "/v1/addresses/" + secondID + "/default", bearer: s.token})
	expectStatus(t, r, http.StatusOK)
	if !decode[addressesService.AddressResponse](t, r).Address.IsDefault {
		t.Fatalf("default not moved")
	}

	f.addToCart(t, s.token, variant, 1)
	r = f.do(t, request{method: "GET", path: "/v1/admin/carts", adminKey: testAdminKey})
	expectStatus(t, r, http.StatusOK)
	carts := decode[adminOperationsService.ListCartsResponse](t, r)
	if carts.Total != 1 || len(carts.Carts[0].Lines) != 1 || !carts.Carts[0].Lines[0].Exists || carts.Carts[0].Value != 1500 {
		t.Fatalf("carts %+v", carts)
	}
	body, contentType := multipartBody(t, "file", map[string][]byte{"look.png": pngBytes(t, 16)}, nil)
	expectStatus(t, f.do(t, request{method: "POST", path: "/v1/admin/images", adminKey: testAdminKey, raw: body, contentType: contentType}), http.StatusCreated)
	r = f.do(t, request{method: "GET", path: "/v1/admin/images", adminKey: testAdminKey})
	if images := decode[adminOperationsService.ListImagesResponse](t, r); images.Total != 1 || images.Images[0].Width == 0 {
		t.Fatalf("images %+v", images)
	}
	r = f.do(t, request{method: "GET", path: "/v1/admin/products", adminKey: testAdminKey})
	if products := decode[adminOperationsService.ListProductsResponse](t, r); products.Total != 1 || products.Products[0].TotalStock != 4 {
		t.Fatalf("products %+v", products)
	}

	s.addressID = secondID
	order := f.placeOrder(t, s, utils.NewULID())
	if order.ShippingAddress.AddressID != secondID || order.ShippingFee != 0 {
		t.Fatalf("order must ship to the chosen address: %+v", order.ShippingAddress)
	}
	r = f.do(t, request{method: "POST", path: "/v1/orders/" + order.ID + "/pay", bearer: s.token})
	expectStatus(t, r, http.StatusOK)
	if paid := decode[ordersService.OrderResponse](t, r).Order; paid.Payment == nil || paid.Payment.ID != order.Payment.ID || paid.Payment.Action == nil {
		t.Fatalf("pay must return the open payment and its instructions: %+v", paid.Payment)
	}
	r = f.do(t, request{method: "GET", path: "/v1/orders?states=CREATED", bearer: s.token})
	if list := decode[ordersService.ListOrdersResponse](t, r).Orders; len(list) != 1 {
		t.Fatalf("state filter returned %d", len(list))
	}
	expectError(t, f.do(t, request{method: "GET", path: "/v1/orders?states=NOPE", bearer: s.token}), http.StatusBadRequest, "state_invalid")

	r = f.do(t, request{method: "POST", path: "/v1/admin/orders/" + order.ID + "/transition", adminKey: testAdminKey, body: `{"to":"CANCELLED","reason":"out of fabric"}`})
	expectStatus(t, r, http.StatusOK)
	cancelled := decode[ordersService.OrderResponse](t, r).Order
	if cancelled.State != "CANCELLED" || cancelled.Payments[0].State != "FAILED" {
		t.Fatalf("admin cancel %+v", cancelled)
	}
	f.orders.Wait()
	if f.stock(t, variant) != 4 {
		t.Fatalf("admin cancel must restore stock")
	}
	if !hasSubject(f.mailsTo("views@example.com"), "was cancelled") {
		t.Fatalf("no cancellation email")
	}
	r = f.do(t, request{method: "POST", path: "/v1/admin/orders/" + order.ID + "/transition", adminKey: testAdminKey, body: `{"to":"REFUNDED"}`})
	expectStatus(t, r, http.StatusOK)
	f.orders.Wait()
	if !hasSubject(f.mailsTo("views@example.com"), "Refund for order") {
		t.Fatalf("no refund email")
	}
	expectError(t, f.do(t, request{method: "GET", path: "/v1/admin/payments/" + order.Payment.ID + "/proof", adminKey: testAdminKey}), http.StatusNotFound, "proof_not_found")
	expectError(t, f.do(t, request{method: "GET", path: "/v1/admin/users/" + utils.NewULID(), adminKey: testAdminKey}), http.StatusNotFound, "user_not_found")
	expectError(t, f.do(t, request{method: "GET", path: "/v1/admin/payments?states=BOGUS", adminKey: testAdminKey}), http.StatusBadRequest, "state_invalid")
}

func TestOrderTrackingStepsAndMe(t *testing.T) {
	f := newFixture(t)
	variant := f.createStockedProduct(t, "TRACK-1", 1200, 3)
	s := f.shopper(t, "tracker@example.com")

	r := f.do(t, request{method: "GET", path: "/v1/users/me", bearer: s.token})
	expectStatus(t, r, http.StatusOK)
	me := decode[usersService.VerifyAccessTokenResponse](t, r)
	if me.Name != "A" || me.Email != "tracker@example.com" || me.IsEmailValidated {
		t.Fatalf("me %+v", me)
	}
	expectError(t, f.do(t, request{method: "GET", path: "/v1/users/me"}), http.StatusUnauthorized, "unauthorized")

	f.addToCart(t, s.token, variant, 1)
	order := f.placeOrder(t, s, utils.NewULID())
	if len(order.Steps) != 6 || !order.Steps[0].Done || !order.Steps[0].Current || order.Steps[1].Done || order.Closed {
		t.Fatalf("steps on a new order %+v", order.Steps)
	}
	expectStatus(t, f.submitProof(t, s.token, order.ID, "412300000099"), http.StatusOK)
	expectStatus(t, f.do(t, request{method: "POST", path: "/v1/admin/payments/" + order.Payment.ID + "/decision", adminKey: testAdminKey, body: `{"status":"SUCCEEDED"}`}), http.StatusOK)

	tracking := `{"courier":"Delhivery","number":"DL123456789","url":"https://www.delhivery.com/track/package/DL123456789"}`
	expectError(t, f.do(t, request{method: "POST", path: "/v1/admin/orders/" + order.ID + "/tracking", adminKey: testAdminKey, body: `{"courier":"X","number":"1","url":"javascript:alert(1)"}`}), http.StatusBadRequest, "tracking_invalid")
	expectError(t, f.do(t, request{method: "POST", path: "/v1/admin/orders/" + order.ID + "/tracking", adminKey: testAdminKey, body: `{"courier":"","number":"1","url":"https://x.com"}`}), http.StatusBadRequest, "tracking_invalid")
	r = f.do(t, request{method: "POST", path: "/v1/admin/orders/" + order.ID + "/tracking", adminKey: testAdminKey, body: tracking})
	expectStatus(t, r, http.StatusOK)
	if got := decode[ordersService.OrderResponse](t, r).Order.Tracking; got == nil || got.Courier != "Delhivery" || got.Number != "DL123456789" {
		t.Fatalf("admin tracking %+v", got)
	}
	for _, to := range []string{"PACKED", "STARTED_SHIPPING", "OUT_FOR_DELIVERY"} {
		expectStatus(t, f.do(t, request{method: "POST", path: "/v1/admin/orders/" + order.ID + "/transition", adminKey: testAdminKey, body: `{"to":"` + to + `"}`}), http.StatusOK)
	}
	f.orders.Wait()
	order = f.getOrder(t, s.token, order.ID)
	if order.State != "OUT_FOR_DELIVERY" || order.Tracking == nil || order.Tracking.URL != "https://www.delhivery.com/track/package/DL123456789" || order.Metadata != nil {
		t.Fatalf("customer order %+v", order)
	}
	for i, step := range order.Steps {
		if step.Done != (i <= 4) || step.Current != (i == 4) || (step.Done && step.At == nil) {
			t.Fatalf("step %d %+v", i, step)
		}
	}
	subjects := f.mailsTo("tracker@example.com")
	for _, want := range []string{"is packed", "has shipped", "is out for delivery"} {
		if !hasSubject(subjects, want) {
			t.Fatalf("missing %q email in %v", want, subjects)
		}
	}
	f.mailer.mu.Lock()
	foundLink := false
	for _, mail := range f.mailer.sent {
		if strings.Contains(mail.Subject, "has shipped") && strings.Contains(mail.Body, "DL123456789") {
			foundLink = true
		}
	}
	f.mailer.mu.Unlock()
	if !foundLink {
		t.Fatalf("shipped email must include the tracking details")
	}
	expectStatus(t, f.do(t, request{method: "POST", path: "/v1/admin/orders/" + order.ID + "/tracking", adminKey: testAdminKey, body: `{}`}), http.StatusOK)
	if f.getOrder(t, s.token, order.ID).Tracking != nil {
		t.Fatalf("empty body must clear tracking")
	}
	expectStatus(t, f.do(t, request{method: "POST", path: "/v1/admin/orders/" + order.ID + "/transition", adminKey: testAdminKey, body: `{"to":"DELIVERED"}`}), http.StatusOK)
	order = f.getOrder(t, s.token, order.ID)
	if !order.Steps[5].Done || !order.Steps[5].Current {
		t.Fatalf("delivered step %+v", order.Steps[5])
	}

	f.addToCart(t, s.token, variant, 1)
	cancelled := f.placeOrder(t, s, utils.NewULID())
	r = f.do(t, request{method: "POST", path: "/v1/orders/" + cancelled.ID + "/cancel", bearer: s.token})
	if got := decode[ordersService.OrderResponse](t, r).Order; !got.Closed || got.Steps[0].Current {
		t.Fatalf("cancelled order must be closed with no current step: %+v", got.Steps)
	}
}
