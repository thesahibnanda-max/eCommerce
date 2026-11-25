package handler_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	selfUPIClient "github.com/thesahibnanda-max/eCommerce/backend/package/clients/self_upi"
	"github.com/thesahibnanda-max/eCommerce/backend/package/handler"
	"github.com/thesahibnanda-max/eCommerce/backend/package/otp"
	paymentGateway "github.com/thesahibnanda-max/eCommerce/backend/package/payment_gateway"
	rateLimiter "github.com/thesahibnanda-max/eCommerce/backend/package/rate_limiter"
	addressRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/address"
	cartsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/carts"
	imagesRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/images"
	ordersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/orders"
	paymentsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/payments"
	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
	usersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/users"
	addressesService "github.com/thesahibnanda-max/eCommerce/backend/package/services/addresses"
	adminService "github.com/thesahibnanda-max/eCommerce/backend/package/services/admin"
	adminOperationsService "github.com/thesahibnanda-max/eCommerce/backend/package/services/admin_operations"
	cartsService "github.com/thesahibnanda-max/eCommerce/backend/package/services/carts"
	imagesService "github.com/thesahibnanda-max/eCommerce/backend/package/services/images"
	ordersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/orders"
	productsService "github.com/thesahibnanda-max/eCommerce/backend/package/services/products"
	staticInfoService "github.com/thesahibnanda-max/eCommerce/backend/package/services/static_info"
	usersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/users"
	"github.com/thesahibnanda-max/eCommerce/backend/package/smtp"
	staticInfo "github.com/thesahibnanda-max/eCommerce/backend/package/static_info"
	"github.com/thesahibnanda-max/eCommerce/backend/package/token"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	testSecret   = "0123456789abcdef0123456789abcdef"
	testAdminKey = "admin-secret-key"
	testPassword = "Secr3t!pass"
)

type fakeMailer struct {
	mu   sync.Mutex
	sent []smtp.SendMailRequest
}

func (m *fakeMailer) SendMail(_ context.Context, req smtp.SendMailRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, req)
	return nil
}

func (m *fakeMailer) lastCode(t *testing.T) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sent) == 0 {
		t.Fatal("no otp mail sent")
	}
	code, _, _ := strings.Cut(strings.TrimPrefix(m.sent[len(m.sent)-1].Body, "Your OTP is: "), "\n")
	return code
}

func newApp(h handler.Handler) *fiber.App {
	app := fiber.New(fiber.Config{BodyLimit: 64 << 20})
	v1 := app.Group("/v1")
	v1.Post("/users/signup", h.SignUp)
	v1.Post("/users/login", h.Login)
	v1.Post("/users/token/refresh", h.RefreshToken)
	v1.Post("/users/email/validation", h.StartEmailValidation)
	v1.Post("/users/email/validate", h.ValidateEmail)
	v1.Get("/users/me", h.GetMe)
	v1.Get("/products", h.ListProducts)
	v1.Get("/products/search", h.SearchProducts)
	v1.Get("/products/categories", h.ListCategories)
	v1.Get("/products/categories/:category", h.ListProductsByCategory)
	v1.Get("/products/sku/:sku", h.GetProductBySKU)
	v1.Get("/products/variants/:variantID", h.GetProductByVariantID)
	v1.Post("/products/batch-get", h.GetProductsByIDs)
	v1.Get("/products/:id", h.GetProductByID)
	v1.Get("/products/:id/recommendations", h.RecommendProducts)
	v1.Post("/images/info/batch", h.GetImagesInfo)
	v1.Get("/images/:id", h.GetImage)
	v1.Get("/images/:id/info", h.GetImageInfo)
	v1.Get("/cart", h.GetCart)
	v1.Delete("/cart", h.ClearCart)
	v1.Post("/cart/items", h.AddCartItem)
	v1.Post("/cart/merge", h.MergeCart)
	v1.Get("/info/pages", h.ListInfoPages)
	v1.Get("/info/pages/:slug", h.GetInfoPage)
	v1.Put("/cart/items/:variantID", h.SetCartItemQuantity)
	v1.Delete("/cart/items/:variantID", h.RemoveCartItem)
	v1.Post("/admin/products", h.AdminCreateProduct)
	v1.Post("/admin/products/batch", h.AdminCreateProducts)
	v1.Put("/admin/products/batch", h.AdminUpdateProducts)
	v1.Put("/admin/products/:id", h.AdminUpdateProduct)
	v1.Post("/admin/images", h.AdminCreateImage)
	v1.Post("/admin/images/batch", h.AdminCreateImages)
	v1.Post("/admin/images/batch-delete", h.AdminDeleteImages)
	v1.Delete("/admin/images/:id", h.AdminDeleteImage)
	v1.Get("/admin/overview", h.AdminOverview)
	v1.Get("/admin/users", h.AdminListUsers)
	v1.Get("/admin/users/:id", h.AdminGetUser)
	v1.Get("/admin/carts", h.AdminListCarts)
	v1.Get("/admin/products", h.AdminListProducts)
	v1.Get("/admin/images", h.AdminListImages)
	v1.Get("/admin/orders", h.AdminListOrders)
	v1.Get("/admin/orders/:id", h.AdminGetOrder)
	v1.Post("/admin/orders/:id/transition", h.AdminTransitionOrder)
	v1.Post("/admin/orders/:id/tracking", h.AdminSetTracking)
	v1.Get("/admin/payments", h.AdminListPayments)
	v1.Get("/admin/payments/:id", h.AdminGetPayment)
	v1.Get("/admin/payments/:id/proof", h.AdminGetPaymentProof)
	v1.Post("/admin/payments/:id/decision", h.AdminDecidePayment)
	v1.Get("/addresses", h.ListAddresses)
	v1.Post("/addresses", h.CreateAddress)
	v1.Put("/addresses/:id", h.UpdateAddress)
	v1.Delete("/addresses/:id", h.DeleteAddress)
	v1.Post("/addresses/:id/default", h.SetDefaultAddress)
	v1.Get("/checkout/quote", h.GetCheckoutQuote)
	v1.Get("/orders", h.ListOrders)
	v1.Post("/orders", h.PlaceOrder)
	v1.Get("/orders/:id", h.GetOrder)
	v1.Post("/orders/:id/pay", h.PayOrder)
	v1.Post("/orders/:id/payment-proof", h.SubmitPaymentProof)
	v1.Post("/orders/:id/cancel", h.CancelOrder)
	v1.Post("/payments/webhook", h.PaymentWebhook)
	v1.Get("/payments/redirect", h.PaymentRedirect)
	return app
}

type fixture struct {
	app      *fiber.App
	mailer   *fakeMailer
	orders   ordersService.Service
	products productsRepository.Repository
}

func newLimiter(t *testing.T, rules map[string]rateLimiter.Rule) rateLimiter.Limiter {
	t.Helper()
	all := map[string]rateLimiter.Rule{}
	for _, group := range []string{"auth", "otp", "catalog", "images", "cart", "admin", "info", "orders", "payments"} {
		all[group] = rateLimiter.Rule{RequestsPerMinute: 6000, Burst: 1000}
	}
	for group, rule := range rules {
		all[group] = rule
	}
	l, err := rateLimiter.NewLimiter(rateLimiter.Config{CacheSize: 100, TTL: time.Hour, Rules: all})
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	return newFixtureWithLimits(t, nil)
}

func newFixtureWithLimits(t *testing.T, rules map[string]rateLimiter.Rule) fixture {
	t.Helper()
	dir := t.TempDir()
	usersRepo, err := usersRepository.NewRepository([]string{dir, "users.db"}, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = usersRepo.Close() })
	productsRepo, err := productsRepository.NewRepository([]string{dir, "products.db"}, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = productsRepo.Close() })
	imagesRepo, err := imagesRepository.NewRepository([]string{dir, "images.db"}, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = imagesRepo.Close() })

	tk, err := token.NewTokenizer(testSecret)
	if err != nil {
		t.Fatal(err)
	}
	mailer := &fakeMailer{}
	emailOTP, err := otp.NewMailOTP(mailer, otp.Config{})
	if err != nil {
		t.Fatal(err)
	}
	usersSvc, err := usersService.NewService(tk, usersRepo, emailOTP, true)
	if err != nil {
		t.Fatal(err)
	}
	adminSvc, err := adminService.NewService(testAdminKey, productsRepo, imagesRepo)
	if err != nil {
		t.Fatal(err)
	}
	productsSvc, err := productsService.NewService(productsRepo)
	if err != nil {
		t.Fatal(err)
	}
	imagesSvc, err := imagesService.NewService(imagesRepo)
	if err != nil {
		t.Fatal(err)
	}
	cartsRepo, err := cartsRepository.NewRepository([]string{dir, "carts.db"}, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cartsRepo.Close() })
	cartsSvc, err := cartsService.NewService(usersSvc, productsRepo, cartsRepo)
	if err != nil {
		t.Fatal(err)
	}
	ordersRepo, err := ordersRepository.NewRepository([]string{dir, "orders.db"}, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ordersRepo.Close() })
	addressRepo, err := addressRepository.NewRepository([]string{dir, "addresses.db"}, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = addressRepo.Close() })
	paymentsRepo, err := paymentsRepository.NewRepository([]string{dir, "payments.db"}, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = paymentsRepo.Close() })
	upiClient, err := selfUPIClient.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	var ordersSvc ordersService.Service
	gateway, err := paymentGateway.NewGateway(paymentGateway.UPIConstructorParams{
		Client:      upiClient,
		Payments:    paymentsRepo,
		PayeeName:   "Test Store",
		OnFinalized: func(ctx context.Context, p paymentGateway.Payment) { ordersSvc.HandlePaymentFinalized(ctx, p) },
	})
	if err != nil {
		t.Fatal(err)
	}
	ordersSvc, err = ordersService.NewService(ordersService.Params{
		Users:             usersSvc,
		UsersRepo:         usersRepo,
		Products:          productsRepo,
		Carts:             cartsRepo,
		Addresses:         addressRepo,
		Orders:            ordersRepo,
		Payments:          paymentsRepo,
		Gateway:           gateway,
		Mailer:            mailer,
		FreeShippingAbove: 999,
		ShippingFee:       79,
		StoreName:         "Test Store",
		StoreURL:          "http://localhost:4073",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ordersSvc.Wait)
	addressesSvc, err := addressesService.NewService(usersSvc, addressRepo)
	if err != nil {
		t.Fatal(err)
	}
	adminOps, err := adminOperationsService.NewService(adminOperationsService.Params{
		AdminKey:        testAdminKey,
		Users:           usersRepo,
		Products:        productsRepo,
		Images:          imagesRepo,
		Carts:           cartsRepo,
		Addresses:       addressRepo,
		Orders:          ordersRepo,
		Payments:        paymentsRepo,
		OrdersService:   ordersSvc,
		PaymentProvider: gateway.Provider().String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	h, err := handler.NewHandler(handler.Services{
		Users:           usersSvc,
		Admin:           adminSvc,
		Products:        productsSvc,
		Images:          imagesSvc,
		Carts:           cartsSvc,
		Info:            newInfoService(t),
		Addresses:       addressesSvc,
		Orders:          ordersSvc,
		AdminOperations: adminOps,
	}, newLimiter(t, rules))
	if err != nil {
		t.Fatal(err)
	}
	return fixture{app: newApp(h), mailer: mailer, orders: ordersSvc, products: productsRepo}
}

type request struct {
	method      string
	path        string
	body        string
	bearer      string
	adminKey    string
	contentType string
	raw         []byte
}

type response struct {
	status  int
	body    []byte
	headers http.Header
}

func (f fixture) do(t *testing.T, r request) response {
	t.Helper()
	var body io.Reader = strings.NewReader(r.body)
	if r.raw != nil {
		body = bytes.NewReader(r.raw)
	}
	req := httptest.NewRequest(r.method, r.path, body)
	ct := r.contentType
	if ct == "" {
		ct = "application/json"
	}
	req.Header.Set("Content-Type", ct)
	if r.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+r.bearer)
	}
	if r.adminKey != "" {
		req.Header.Set("X-Admin-Key", r.adminKey)
	}
	resp, err := f.app.Test(req, fiber.TestConfig{Timeout: 0})
	if err != nil {
		t.Fatalf("%s %s: %v", r.method, r.path, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response{status: resp.StatusCode, body: b, headers: resp.Header}
}

func decode[T any](t *testing.T, r response) T {
	t.Helper()
	v, err := utils.FromStringToObjectJSON[T](string(r.body))
	if err != nil {
		t.Fatalf("decode %s: %v", r.body, err)
	}
	return v
}

type errBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func expectError(t *testing.T, r response, status int, code string) {
	t.Helper()
	if r.status != status {
		t.Fatalf("status = %d, want %d (body %s)", r.status, status, r.body)
	}
	if got := decode[errBody](t, r); got.Code != code {
		t.Fatalf("code = %q, want %q (body %s)", got.Code, code, r.body)
	}
}

func expectStatus(t *testing.T, r response, status int) {
	t.Helper()
	if r.status != status {
		t.Fatalf("status = %d, want %d (body %s)", r.status, status, r.body)
	}
}

var phoneSeq atomic.Uint32

func nextPhone() string {
	return "+9198765" + strconv.FormatUint(uint64(100000+phoneSeq.Add(1)), 10)[1:]
}

func signUpBody(name, email, phone, password string) string {
	return `{"name":"` + name + `","email":"` + email + `","phone_number":"` + phone + `","password":"` + password + `"}`
}

func (f fixture) signUp(t *testing.T, email string) usersService.CreateUserResponse {
	t.Helper()
	r := f.do(t, request{method: "POST", path: "/v1/users/signup", body: signUpBody("A", email, nextPhone(), testPassword)})
	expectStatus(t, r, http.StatusCreated)
	return decode[usersService.CreateUserResponse](t, r)
}

func pngBytes(t *testing.T, size int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 255 / size), G: uint8(y * 255 / size), B: 70, A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func multipartBody(t *testing.T, field string, files map[string][]byte, extra map[string]string) ([]byte, string) {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for name, content := range files {
		part, err := w.CreateFormFile(field, name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	for k, v := range extra {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes(), w.FormDataContentType()
}

func stubServices() handler.Services {
	return handler.Services{
		Users:           stubUsers{},
		Admin:           stubAdmin{},
		Products:        stubProducts{},
		Images:          stubImages{},
		Carts:           stubCarts{},
		Info:            stubInfo{},
		Addresses:       stubAddresses{},
		Orders:          stubOrders{},
		AdminOperations: stubAdminOps{},
	}
}

func TestNewHandler_RequiresServices(t *testing.T) {
	limiter := newLimiter(t, nil)
	tests := []struct {
		name    string
		mutate  func(*handler.Services)
		wantErr error
	}{
		{"users", func(s *handler.Services) { s.Users = nil }, handler.ErrUsersServiceRequired},
		{"admin", func(s *handler.Services) { s.Admin = nil }, handler.ErrAdminServiceRequired},
		{"products", func(s *handler.Services) { s.Products = nil }, handler.ErrProductsServiceRequired},
		{"images", func(s *handler.Services) { s.Images = nil }, handler.ErrImagesServiceRequired},
		{"carts", func(s *handler.Services) { s.Carts = nil }, handler.ErrCartsServiceRequired},
		{"info", func(s *handler.Services) { s.Info = nil }, handler.ErrStaticInfoServiceRequired},
		{"addresses", func(s *handler.Services) { s.Addresses = nil }, handler.ErrAddressesServiceRequired},
		{"orders", func(s *handler.Services) { s.Orders = nil }, handler.ErrOrdersServiceRequired},
		{"admin operations", func(s *handler.Services) { s.AdminOperations = nil }, handler.ErrAdminOpsServiceRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services := stubServices()
			tt.mutate(&services)
			if _, err := handler.NewHandler(services, limiter); !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
	if _, err := handler.NewHandler(stubServices(), nil); !errors.Is(err, handler.ErrRateLimiterRequired) {
		t.Fatalf("nil limiter err = %v", err)
	}
	partial, err := rateLimiter.NewLimiter(rateLimiter.Config{CacheSize: 10, TTL: time.Hour, Rules: map[string]rateLimiter.Rule{"auth": {RequestsPerMinute: 60, Burst: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handler.NewHandler(stubServices(), partial); !errors.Is(err, handler.ErrRateLimitGroupMissing) {
		t.Fatalf("partial limiter err = %v, want ErrRateLimitGroupMissing", err)
	}
}

func TestUserFlow(t *testing.T) {
	f := newFixture(t)
	created := f.signUp(t, "a@example.com")
	if !utils.ValidateULID(created.ID) || created.AccessToken == "" || created.RefreshToken == "" {
		t.Fatalf("signup = %+v", created)
	}

	expectError(t, f.do(t, request{method: "POST", path: "/v1/users/signup", body: signUpBody("B", "a@example.com", nextPhone(), testPassword)}), http.StatusConflict, "email_already_exists")
	expectError(t, f.do(t, request{method: "POST", path: "/v1/users/signup", body: signUpBody("B", "b@example.com", nextPhone(), "short")}), http.StatusBadRequest, "password_invalid")
	expectError(t, f.do(t, request{method: "POST", path: "/v1/users/signup", body: `{"name":"B","email":"b@example.com","password":"` + testPassword + `"}`}), http.StatusBadRequest, "phone_number_required")
	expectError(t, f.do(t, request{method: "POST", path: "/v1/users/signup", body: `{"name":"B","phone_number":"+919812345678","password":"` + testPassword + `"}`}), http.StatusBadRequest, "email_required")
	expectError(t, f.do(t, request{method: "POST", path: "/v1/users/signup", body: signUpBody("B", "b@example.com", "123", testPassword)}), http.StatusBadRequest, "phone_number_invalid")
	dupPhone := nextPhone()
	expectStatus(t, f.do(t, request{method: "POST", path: "/v1/users/signup", body: signUpBody("C", "c@example.com", dupPhone, testPassword)}), http.StatusCreated)
	expectError(t, f.do(t, request{method: "POST", path: "/v1/users/signup", body: signUpBody("D", "d@example.com", dupPhone, testPassword)}), http.StatusConflict, "phone_number_already_exists")
	phoneLogin := f.do(t, request{method: "POST", path: "/v1/users/login", body: `{"phone_number":"` + dupPhone + `","password":"` + testPassword + `"}`})
	expectStatus(t, phoneLogin, http.StatusOK)
	expectError(t, f.do(t, request{method: "POST", path: "/v1/users/signup", body: `{not json`}), http.StatusBadRequest, "body_invalid")

	login := f.do(t, request{method: "POST", path: "/v1/users/login", body: `{"email":"a@example.com","password":"` + testPassword + `"}`})
	expectStatus(t, login, http.StatusOK)
	tokens := decode[usersService.GetUserAndVerifyPasswordResponse](t, login)
	if tokens.ID != created.ID {
		t.Fatalf("login id = %s, want %s", tokens.ID, created.ID)
	}
	expectError(t, f.do(t, request{method: "POST", path: "/v1/users/login", body: `{"email":"a@example.com","password":"Wr0ng!pass"}`}), http.StatusUnauthorized, "invalid_credentials")
	expectError(t, f.do(t, request{method: "POST", path: "/v1/users/login", body: `{"password":"x"}`}), http.StatusBadRequest, "contact_required")

	refresh := f.do(t, request{method: "POST", path: "/v1/users/token/refresh", body: `{"refresh_token":"` + tokens.RefreshToken + `"}`})
	expectStatus(t, refresh, http.StatusOK)
	refreshed := decode[usersService.RefreshAccessTokenResponse](t, refresh)
	expectError(t, f.do(t, request{method: "POST", path: "/v1/users/token/refresh", body: `{"refresh_token":"` + tokens.RefreshToken + `"}`}), http.StatusUnauthorized, "token_reused")

	expectStatus(t, f.do(t, request{method: "GET", path: "/v1/cart", bearer: refreshed.AccessToken}), http.StatusOK)
	expectError(t, f.do(t, request{method: "GET", path: "/v1/cart"}), http.StatusUnauthorized, "unauthorized")
	expectError(t, f.do(t, request{method: "GET", path: "/v1/cart", bearer: "garbage"}), http.StatusUnauthorized, "token_invalid")
	expectError(t, f.do(t, request{method: "GET", path: "/v1/cart", bearer: refreshed.RefreshToken}), http.StatusUnauthorized, "token_wrong_type")

	expectStatus(t, f.do(t, request{method: "POST", path: "/v1/users/email/validation", bearer: refreshed.AccessToken}), http.StatusAccepted)
	expectError(t, f.do(t, request{method: "POST", path: "/v1/users/email/validation", bearer: refreshed.AccessToken}), http.StatusTooManyRequests, "otp_resend_too_soon")
	expectError(t, f.do(t, request{method: "POST", path: "/v1/users/email/validate", bearer: refreshed.AccessToken, body: `{"otp":"000000x"}`}), http.StatusBadRequest, "otp_invalid")
	validated := f.do(t, request{method: "POST", path: "/v1/users/email/validate", bearer: refreshed.AccessToken, body: `{"otp":"` + f.mailer.lastCode(t) + `"}`})
	expectStatus(t, validated, http.StatusOK)
	if got := decode[usersService.ValidateEmailResponse](t, validated); !got.IsValidated {
		t.Fatalf("validate = %+v", got)
	}
	expectError(t, f.do(t, request{method: "POST", path: "/v1/users/email/validation", bearer: refreshed.AccessToken}), http.StatusConflict, "email_already_validated")
}

func TestAdminAndProductRoutes(t *testing.T) {
	f := newFixture(t)
	user := f.signUp(t, "shopper@example.com")
	product := `{"name":"Cotton Shirt","categories":["shirts"],"variants":[{"sku":"CS-1","shown_price":500,"selling_price":400,"currency":"INR","stock_units":3}]}`

	expectError(t, f.do(t, request{method: "POST", path: "/v1/admin/products", body: product}), http.StatusUnauthorized, "unauthorized")
	expectError(t, f.do(t, request{method: "POST", path: "/v1/admin/products", body: product, adminKey: "wrong"}), http.StatusUnauthorized, "admin_unauthorized")

	created := f.do(t, request{method: "POST", path: "/v1/admin/products", body: product, adminKey: testAdminKey})
	expectStatus(t, created, http.StatusCreated)
	shirt := decode[adminService.CreateProductResponse](t, created).Product

	batch := `{"products":[
		{"name":"Linen Shirt","categories":["shirts"],"variants":[{"sku":"LS-1","shown_price":900,"selling_price":900,"currency":"INR","stock_units":0}]},
		{"name":"Denim Jeans","categories":["jeans"],"variants":[{"sku":"DJ-1","shown_price":1500,"selling_price":1500,"currency":"INR","stock_units":2}]}
	]}`
	expectStatus(t, f.do(t, request{method: "POST", path: "/v1/admin/products/batch", body: batch, adminKey: testAdminKey}), http.StatusCreated)

	dupBatch := `{"products":[
		{"name":"X","variants":[{"sku":"NEW-1","shown_price":1,"selling_price":1,"currency":"INR"}]},
		{"name":"Y","variants":[{"sku":"CS-1","shown_price":1,"selling_price":1,"currency":"INR"}]}
	]}`
	expectError(t, f.do(t, request{method: "POST", path: "/v1/admin/products/batch", body: dupBatch, adminKey: testAdminKey}), http.StatusConflict, "sku_already_exists")
	expectError(t, f.do(t, request{method: "POST", path: "/v1/admin/products/batch", body: `{"products":[]}`, adminKey: testAdminKey}), http.StatusBadRequest, "batch_empty")
	expectError(t, f.do(t, request{method: "POST", path: "/v1/admin/products", body: `{"name":"","variants":[]}`, adminKey: testAdminKey}), http.StatusBadRequest, "name_required")

	update := `{"name":"Cotton Shirt v2","categories":["shirts"],"variants":[{"id":"` + shirt.Variants[0].ID + `","sku":"CS-1","shown_price":500,"selling_price":350,"currency":"INR","stock_units":3}]}`
	updated := f.do(t, request{method: "PUT", path: "/v1/admin/products/" + shirt.ID, body: update, adminKey: testAdminKey})
	expectStatus(t, updated, http.StatusOK)
	if got := decode[adminService.UpdateProductResponse](t, updated).Product; got.ID != shirt.ID || got.Name != "Cotton Shirt v2" {
		t.Fatalf("updated = %+v", got)
	}
	expectError(t, f.do(t, request{method: "PUT", path: "/v1/admin/products/" + utils.NewULID(), body: update, adminKey: testAdminKey}), http.StatusNotFound, "product_not_found")
	batchUpdate := `{"products":[{"id":"` + shirt.ID + `","name":"Cotton Shirt v3","categories":["shirts"],"variants":[{"id":"` + shirt.Variants[0].ID + `","sku":"CS-1","shown_price":500,"selling_price":350,"currency":"INR","stock_units":3}]}]}`
	expectStatus(t, f.do(t, request{method: "PUT", path: "/v1/admin/products/batch", body: batchUpdate, adminKey: testAdminKey}), http.StatusOK)

	tk := user.AccessToken
	expectStatus(t, f.do(t, request{method: "GET", path: "/v1/products"}), http.StatusOK)
	expectStatus(t, f.do(t, request{method: "GET", path: "/v1/products", bearer: "garbage"}), http.StatusOK)

	list := decode[productsService.ListProductsResponse](t, f.do(t, request{method: "GET", path: "/v1/products", bearer: tk}))
	if list.Total != 3 || len(list.Products) != 3 {
		t.Fatalf("list total %d len %d", list.Total, len(list.Products))
	}
	paged := decode[productsService.ListProductsResponse](t, f.do(t, request{method: "GET", path: "/v1/products?offset=1&limit=1", bearer: tk}))
	if paged.Total != 3 || len(paged.Products) != 1 {
		t.Fatalf("paged total %d len %d", paged.Total, len(paged.Products))
	}
	expectError(t, f.do(t, request{method: "GET", path: "/v1/products?limit=abc", bearer: tk}), http.StatusBadRequest, "query_invalid")

	search := decode[productsService.SearchProductsResponse](t, f.do(t, request{method: "GET", path: "/v1/products/search?text=shirt&sort=price_asc&in_stock_only=false", bearer: tk}))
	if search.Total != 2 || search.Products[0].Name != "Cotton Shirt v3" {
		t.Fatalf("search = %+v", search)
	}
	filtered := decode[productsService.SearchProductsResponse](t, f.do(t, request{method: "GET", path: "/v1/products/search?categories=jeans,shirts&min_price=1000&max_price=2000", bearer: tk}))
	if filtered.Total != 1 || filtered.Products[0].Name != "Denim Jeans" {
		t.Fatalf("filtered = %+v", filtered)
	}
	expectError(t, f.do(t, request{method: "GET", path: "/v1/products/search?min_price=x", bearer: tk}), http.StatusBadRequest, "query_invalid")
	expectError(t, f.do(t, request{method: "GET", path: "/v1/products/search?sort=cheap", bearer: tk}), http.StatusBadRequest, "sort_invalid")
	expectError(t, f.do(t, request{method: "GET", path: "/v1/products/search?in_stock_only=maybe", bearer: tk}), http.StatusBadRequest, "query_invalid")

	cats := decode[productsService.ListCategoriesResponse](t, f.do(t, request{method: "GET", path: "/v1/products/categories", bearer: tk}))
	if len(cats.Categories) != 2 {
		t.Fatalf("categories = %+v", cats)
	}
	byCat := decode[productsService.ListProductsByCategoryResponse](t, f.do(t, request{method: "GET", path: "/v1/products/categories/shirts", bearer: tk}))
	if byCat.Total != 2 {
		t.Fatalf("by category total = %d", byCat.Total)
	}

	bySKU := decode[productsService.GetProductBySKUResponse](t, f.do(t, request{method: "GET", path: "/v1/products/sku/CS-1", bearer: tk}))
	if bySKU.Product.ID != shirt.ID || bySKU.Variant.SKU != "CS-1" {
		t.Fatalf("by sku = %+v", bySKU)
	}
	expectError(t, f.do(t, request{method: "GET", path: "/v1/products/sku/NOPE", bearer: tk}), http.StatusNotFound, "variant_not_found")

	byVariant := decode[productsService.GetProductByVariantIDResponse](t, f.do(t, request{method: "GET", path: "/v1/products/variants/" + shirt.Variants[0].ID, bearer: tk}))
	if byVariant.Product.ID != shirt.ID {
		t.Fatalf("by variant = %+v", byVariant)
	}

	byID := decode[productsService.GetProductByIDResponse](t, f.do(t, request{method: "GET", path: "/v1/products/" + shirt.ID, bearer: tk}))
	if byID.Product.Name != "Cotton Shirt v3" {
		t.Fatalf("by id = %+v", byID)
	}
	expectError(t, f.do(t, request{method: "GET", path: "/v1/products/not-an-id", bearer: tk}), http.StatusBadRequest, "id_invalid")
	expectError(t, f.do(t, request{method: "GET", path: "/v1/products/" + utils.NewULID(), bearer: tk}), http.StatusNotFound, "product_not_found")

	many := decode[productsService.GetProductsByIDsResponse](t, f.do(t, request{method: "POST", path: "/v1/products/batch-get", bearer: tk, body: `{"ids":["` + shirt.ID + `"]}`}))
	if len(many.Products) != 1 {
		t.Fatalf("batch get = %+v", many)
	}
	expectError(t, f.do(t, request{method: "POST", path: "/v1/products/batch-get", bearer: tk, body: `{"ids":[]}`}), http.StatusBadRequest, "batch_empty")

	recs := decode[productsService.RecommendProductsResponse](t, f.do(t, request{method: "GET", path: "/v1/products/" + shirt.ID + "/recommendations?limit=5", bearer: tk}))
	if len(recs.Products) == 0 || recs.Products[0].Name != "Linen Shirt" {
		t.Fatalf("recommendations = %+v", recs)
	}
}

func TestImageRoutes(t *testing.T) {
	f := newFixture(t)

	noFile, ct := multipartBody(t, "other", map[string][]byte{"a.png": pngBytes(t, 8)}, nil)
	expectError(t, f.do(t, request{method: "POST", path: "/v1/admin/images", raw: noFile, contentType: ct, adminKey: testAdminKey}), http.StatusBadRequest, "file_required")

	single, ct := multipartBody(t, "file", map[string][]byte{"cat.png": pngBytes(t, 24)}, map[string]string{"metadata": `{"alt":"a cat"}`})
	expectError(t, f.do(t, request{method: "POST", path: "/v1/admin/images", raw: single, contentType: ct}), http.StatusUnauthorized, "unauthorized")
	created := f.do(t, request{method: "POST", path: "/v1/admin/images", raw: single, contentType: ct, adminKey: testAdminKey})
	expectStatus(t, created, http.StatusCreated)
	cat := decode[adminService.CreateImageResponse](t, created).Image
	if cat.FileName != "cat.avif" || cat.Metadata["alt"] != "a cat" || cat.Width != 24 {
		t.Fatalf("created image = %+v", cat)
	}

	badMeta, ct := multipartBody(t, "file", map[string][]byte{"x.png": pngBytes(t, 9)}, map[string]string{"metadata": `[1,2`})
	expectError(t, f.do(t, request{method: "POST", path: "/v1/admin/images", raw: badMeta, contentType: ct, adminKey: testAdminKey}), http.StatusBadRequest, "metadata_invalid")

	junk, ct := multipartBody(t, "file", map[string][]byte{"junk.png": []byte("not an image")}, nil)
	expectError(t, f.do(t, request{method: "POST", path: "/v1/admin/images", raw: junk, contentType: ct, adminKey: testAdminKey}), http.StatusUnsupportedMediaType, "image_unsupported")

	batchBody, ct := multipartBody(t, "files", map[string][]byte{"a.png": pngBytes(t, 10), "b.png": pngBytes(t, 12)}, nil)
	batch := f.do(t, request{method: "POST", path: "/v1/admin/images/batch", raw: batchBody, contentType: ct, adminKey: testAdminKey})
	expectStatus(t, batch, http.StatusCreated)
	images := decode[adminService.CreateImagesResponse](t, batch).Images
	if len(images) != 2 {
		t.Fatalf("batch images = %+v", images)
	}

	raw := f.do(t, request{method: "GET", path: "/v1/images/" + cat.ID})
	expectStatus(t, raw, http.StatusOK)
	if raw.headers.Get("Content-Type") != "image/avif" || !strings.Contains(raw.headers.Get("Cache-Control"), "immutable") {
		t.Fatalf("image headers = %v", raw.headers)
	}
	if len(raw.body) != cat.Size {
		t.Fatalf("image bytes = %d, want %d", len(raw.body), cat.Size)
	}

	info := decode[imagesService.GetImageInfoByIDResponse](t, f.do(t, request{method: "GET", path: "/v1/images/" + cat.ID + "/info"}))
	if info.Image.ID != cat.ID {
		t.Fatalf("info = %+v", info)
	}
	infos := decode[imagesService.GetImagesInfoByIDsResponse](t, f.do(t, request{method: "POST", path: "/v1/images/info/batch", body: `{"ids":["` + images[1].ID + `","` + images[0].ID + `"]}`}))
	if len(infos.Images) != 2 || infos.Images[0].ID != images[1].ID {
		t.Fatalf("batch info = %+v", infos)
	}
	expectError(t, f.do(t, request{method: "POST", path: "/v1/images/info/batch", body: `{"ids":["` + cat.ID + `","` + cat.ID + `"]}`}), http.StatusConflict, "image_duplicate_in_batch")

	expectError(t, f.do(t, request{method: "POST", path: "/v1/admin/images/batch-delete", body: `{"ids":["` + images[0].ID + `","` + utils.NewULID() + `"]}`, adminKey: testAdminKey}), http.StatusNotFound, "image_not_found")
	expectStatus(t, f.do(t, request{method: "GET", path: "/v1/images/" + images[0].ID + "/info"}), http.StatusOK)
	expectStatus(t, f.do(t, request{method: "POST", path: "/v1/admin/images/batch-delete", body: `{"ids":["` + images[0].ID + `","` + images[1].ID + `"]}`, adminKey: testAdminKey}), http.StatusOK)
	expectError(t, f.do(t, request{method: "GET", path: "/v1/images/" + images[0].ID}), http.StatusNotFound, "image_not_found")

	expectStatus(t, f.do(t, request{method: "DELETE", path: "/v1/admin/images/" + cat.ID, adminKey: testAdminKey}), http.StatusOK)
	expectError(t, f.do(t, request{method: "DELETE", path: "/v1/admin/images/" + cat.ID, adminKey: testAdminKey}), http.StatusNotFound, "image_not_found")
	expectError(t, f.do(t, request{method: "GET", path: "/v1/images/bad-id"}), http.StatusBadRequest, "id_invalid")
}

func TestInternalErrorIsHidden(t *testing.T) {
	h, err := handler.NewHandler(stubServices(), newLimiter(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	app := newApp(h)
	resp, err := app.Test(httptest.NewRequest("POST", "/v1/users/signup", strings.NewReader(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusInternalServerError || !strings.Contains(string(b), `"internal_error"`) || strings.Contains(string(b), "secret") {
		t.Fatalf("internal error response = %d %s", resp.StatusCode, b)
	}
}

type stubUsers struct{ usersService.Service }

func (stubUsers) CreateUser(context.Context, usersService.CreateUserRequest) (usersService.CreateUserResponse, error) {
	return usersService.CreateUserResponse{}, errors.New("db secret exploded")
}

type stubAdmin struct{ adminService.Service }
type stubProducts struct{ productsService.Service }
type stubImages struct{ imagesService.Service }
type stubCarts struct{ cartsService.Service }

type stubInfo struct{ staticInfoService.Service }
type stubAddresses struct{ addressesService.Service }
type stubOrders struct{ ordersService.Service }
type stubAdminOps struct{ adminOperationsService.Service }

func newInfoService(t *testing.T) staticInfoService.Service {
	t.Helper()
	info, err := staticInfo.NewStaticInfo(staticInfo.Config{
		CompanyName:       "Aesthetic Indians",
		SupportEmail:      "help@example.com",
		SupportPhone:      "+919876543210",
		Address:           "Jaipur, Rajasthan",
		GrievanceOfficer:  "Asha Rao",
		EffectiveDate:     "2026-11-13",
		ReturnWindowDays:  7,
		RefundDays:        "5–7",
		DispatchDays:      "1–3",
		DeliveryDays:      "5–7",
		FreeShippingAbove: 999,
		ShippingFee:       79,
	})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := staticInfoService.NewService(info)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestInfoRoutes(t *testing.T) {
	f := newFixture(t)
	list := f.do(t, request{method: "GET", path: "/v1/info/pages"})
	expectStatus(t, list, http.StatusOK)
	pages := decode[staticInfoService.ListPagesResponse](t, list)
	if len(pages.Pages) != 4 || pages.Contact.Email != "help@example.com" {
		t.Fatalf("list = %+v", pages)
	}
	for _, summary := range pages.Pages {
		r := f.do(t, request{method: "GET", path: "/v1/info/pages/" + summary.Slug})
		expectStatus(t, r, http.StatusOK)
		got := decode[staticInfoService.GetPageResponse](t, r)
		if got.Page.Slug != summary.Slug || got.Page.Title != summary.Title || len(got.Page.Sections) == 0 {
			t.Errorf("page %s = %+v", summary.Slug, got.Page)
		}
	}
	expectError(t, f.do(t, request{method: "GET", path: "/v1/info/pages/cookie-policy"}), http.StatusNotFound, "page_not_found")
	expectError(t, f.do(t, request{method: "GET", path: "/v1/info/pages/Bad_Slug"}), http.StatusBadRequest, "slug_invalid")
}

func (f fixture) createProduct(t *testing.T, sku string, stock int) adminService.Product {
	t.Helper()
	body := `{"name":"P ` + sku + `","variants":[{"sku":"` + sku + `","shown_price":500,"selling_price":400,"currency":"INR","stock_units":` + strconv.Itoa(stock) + `}]}`
	r := f.do(t, request{method: "POST", path: "/v1/admin/products", body: body, adminKey: testAdminKey})
	expectStatus(t, r, http.StatusCreated)
	return decode[adminService.CreateProductResponse](t, r).Product
}

func TestCartRoutes(t *testing.T) {
	f := newFixture(t)
	tk := f.signUp(t, "cart@example.com").AccessToken
	a := f.createProduct(t, "A-1", 3)
	b := f.createProduct(t, "B-1", 5)
	va, vb := a.Variants[0].ID, b.Variants[0].ID

	empty := decode[cartsService.GetCartResponse](t, f.do(t, request{method: "GET", path: "/v1/cart", bearer: tk}))
	if len(empty.Cart.Items) != 0 {
		t.Fatalf("empty cart = %+v", empty)
	}
	added := f.do(t, request{method: "POST", path: "/v1/cart/items", bearer: tk, body: `{"variant_id":"` + va + `","quantity":2}`})
	expectStatus(t, added, http.StatusOK)
	if got := decode[cartsService.AddItemResponse](t, added).Cart; got.ItemCount != 2 || got.Items[0].Name != "P A-1" || got.Totals[0].SellingPrice != 800 {
		t.Fatalf("added cart = %+v", got)
	}
	expectError(t, f.do(t, request{method: "POST", path: "/v1/cart/items", bearer: tk, body: `{"variant_id":"` + va + `","quantity":2}`}), http.StatusConflict, "out_of_stock")
	expectError(t, f.do(t, request{method: "POST", path: "/v1/cart/items", bearer: tk, body: `{"variant_id":"` + va + `","quantity":0}`}), http.StatusBadRequest, "quantity_invalid")
	expectError(t, f.do(t, request{method: "POST", path: "/v1/cart/items", bearer: tk, body: `{"variant_id":"` + va + `","quantity":11}`}), http.StatusBadRequest, "quantity_too_large")
	expectError(t, f.do(t, request{method: "POST", path: "/v1/cart/items", bearer: tk, body: `{"variant_id":"bad","quantity":1}`}), http.StatusBadRequest, "id_invalid")
	expectError(t, f.do(t, request{method: "POST", path: "/v1/cart/items", bearer: tk, body: `{"variant_id":"` + utils.NewULID() + `","quantity":1}`}), http.StatusNotFound, "variant_not_found")
	expectError(t, f.do(t, request{method: "POST", path: "/v1/cart/items", bearer: tk, body: `{oops`}), http.StatusBadRequest, "body_invalid")

	set := f.do(t, request{method: "PUT", path: "/v1/cart/items/" + va, bearer: tk, body: `{"quantity":3}`})
	expectStatus(t, set, http.StatusOK)
	if got := decode[cartsService.SetItemQuantityResponse](t, set).Cart; got.Items[0].Quantity != 3 {
		t.Fatalf("set cart = %+v", got)
	}
	expectError(t, f.do(t, request{method: "PUT", path: "/v1/cart/items/" + vb, bearer: tk, body: `{"quantity":1}`}), http.StatusNotFound, "cart_item_not_found")

	merged := f.do(t, request{method: "POST", path: "/v1/cart/merge", bearer: tk, body: `{"items":[{"variant_id":"` + vb + `","quantity":2},{"variant_id":"` + utils.NewULID() + `","quantity":1}]}`})
	expectStatus(t, merged, http.StatusOK)
	mergedCart := decode[cartsService.MergeItemsResponse](t, merged)
	if len(mergedCart.Cart.Items) != 2 || len(mergedCart.Skipped) != 1 {
		t.Fatalf("merged = %+v", mergedCart)
	}
	expectError(t, f.do(t, request{method: "POST", path: "/v1/cart/merge", bearer: tk, body: `{"items":[]}`}), http.StatusBadRequest, "batch_empty")

	removed := f.do(t, request{method: "DELETE", path: "/v1/cart/items/" + va, bearer: tk})
	expectStatus(t, removed, http.StatusOK)
	if got := decode[cartsService.RemoveItemResponse](t, removed).Cart; len(got.Items) != 1 || got.Items[0].VariantID != vb {
		t.Fatalf("removed cart = %+v", got)
	}
	cleared := f.do(t, request{method: "DELETE", path: "/v1/cart", bearer: tk})
	expectStatus(t, cleared, http.StatusOK)
	if got := decode[cartsService.ClearCartResponse](t, cleared).Cart; len(got.Items) != 0 {
		t.Fatalf("cleared cart = %+v", got)
	}
	for _, r := range []request{
		{method: "GET", path: "/v1/cart"},
		{method: "POST", path: "/v1/cart/items", body: `{}`},
		{method: "PUT", path: "/v1/cart/items/" + va, body: `{}`},
		{method: "DELETE", path: "/v1/cart/items/" + va},
		{method: "DELETE", path: "/v1/cart"},
		{method: "POST", path: "/v1/cart/merge", body: `{}`},
	} {
		expectError(t, f.do(t, r), http.StatusUnauthorized, "unauthorized")
	}
}

func TestRateLimit(t *testing.T) {
	f := newFixtureWithLimits(t, map[string]rateLimiter.Rule{
		"auth":    {RequestsPerMinute: 1, Burst: 2},
		"catalog": {RequestsPerMinute: 60, Burst: 1},
	})
	login := request{method: "POST", path: "/v1/users/login", body: `{"email":"x@example.com","password":"` + testPassword + `"}`}
	expectError(t, f.do(t, login), http.StatusUnauthorized, "invalid_credentials")
	expectError(t, f.do(t, login), http.StatusUnauthorized, "invalid_credentials")
	limited := f.do(t, login)
	expectError(t, limited, http.StatusTooManyRequests, "rate_limited")
	if got := limited.headers.Get("Retry-After"); got != "60" {
		t.Fatalf("Retry-After = %q, want 60", got)
	}
	expectError(t, f.do(t, request{method: "POST", path: "/v1/users/signup", body: `{}`}), http.StatusTooManyRequests, "rate_limited")
	expectStatus(t, f.do(t, request{method: "GET", path: "/v1/products"}), http.StatusOK)
	expectError(t, f.do(t, request{method: "GET", path: "/v1/products/categories"}), http.StatusTooManyRequests, "rate_limited")
	expectStatus(t, f.do(t, request{method: "GET", path: "/v1/images/" + utils.NewULID() + "/info"}), http.StatusNotFound)
}
