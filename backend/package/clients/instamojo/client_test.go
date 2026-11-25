package instamojoClient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testClientID     = "test-client-id"
	testClientSecret = "test-client-secret"
	successBody      = `{"id":"05f317448ad84649aa1a9c7328edb015","user":"https://api.instamojo.com/v2/users/90f/","phone":"+919999999999","email":"foo@example.com","buyer_name":"John Doe","amount":"2500.00","purpose":"Order 1","status":"Pending","payments":[],"send_sms":false,"send_email":true,"sms_status":"Pending","email_status":"Pending","shorturl":null,"longurl":"https://www.instamojo.com/@foo/05f317448ad84649aa1a9c7328edb015","redirect_url":"https://shop.example.com/r/","webhook":"https://shop.example.com/w/","created_at":"2016-05-09T16:10:13.786Z","modified_at":"2016-05-09T16:10:13.786Z","resource_uri":"https://api.instamojo.com/v2/payment_requests/05f317448ad84649aa1a9c7328edb015/","allow_repeated_payments":false,"mark_fulfilled":true}`
)

type fakeInstamojo struct {
	t             *testing.T
	tokenCalls    atomic.Int64
	createCalls   atomic.Int64
	tokenDelay    time.Duration
	tokenStatus   int
	tokenBody     func(n int64) string
	createHandler func(w http.ResponseWriter, r *http.Request, n int64)
	getHandler    func(w http.ResponseWriter, r *http.Request)
	getCalls      atomic.Int64
	lastPath      string
	mu            sync.Mutex
	lastForm      url.Values
	lastAuth      string
}

func newFake(t *testing.T) *fakeInstamojo {
	return &fakeInstamojo{
		t:           t,
		tokenStatus: http.StatusOK,
		tokenBody: func(n int64) string {
			return fmt.Sprintf(`{"access_token":"token-%d","token_type":"Bearer","expires_in":36000,"scope":"read write"}`, n)
		},
		createHandler: func(w http.ResponseWriter, _ *http.Request, _ int64) {
			_, _ = w.Write([]byte(successBody))
		},
	}
}

func (f *fakeInstamojo) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		f.getCalls.Add(1)
		f.mu.Lock()
		f.lastAuth = r.Header.Get("Authorization")
		f.lastPath = r.URL.EscapedPath()
		f.mu.Unlock()
		if f.getHandler == nil {
			http.NotFound(w, r)
			return
		}
		f.getHandler(w, r)
		return
	}
	if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
		http.Error(w, "bad request shape", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch r.URL.Path {
	case tokenPath:
		n := f.tokenCalls.Add(1)
		if f.tokenDelay > 0 {
			time.Sleep(f.tokenDelay)
		}
		if r.PostForm.Get("grant_type") != "client_credentials" || r.PostForm.Get("client_id") != testClientID || r.PostForm.Get("client_secret") != testClientSecret {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid_client"}`))
			return
		}
		w.WriteHeader(f.tokenStatus)
		_, _ = w.Write([]byte(f.tokenBody(n)))
	case paymentRequestPath:
		n := f.createCalls.Add(1)
		f.mu.Lock()
		f.lastForm = r.PostForm
		f.lastAuth = r.Header.Get("Authorization")
		f.mu.Unlock()
		f.createHandler(w, r, n)
	default:
		http.NotFound(w, r)
	}
}

func setup(t *testing.T, f *fakeInstamojo) (*client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(f)
	t.Cleanup(server.Close)
	c, err := NewClient(server.URL+"/", testClientID, testClientSecret, 5*time.Second)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c.(*client), server
}

func validRequest() CreatePaymentRequestRequest {
	return CreatePaymentRequestRequest{
		Amount:      2500,
		Purpose:     "  Order   1 ",
		BuyerName:   "John Doe",
		Email:       " Foo@Example.com ",
		Phone:       "99999 99999",
		RedirectURL: "https://shop.example.com/r/",
		WebhookURL:  "https://shop.example.com/w/",
		SendEmail:   true,
	}
}

func TestNewClient_Config(t *testing.T) {
	bad := []struct{ base, id, secret string }{
		{"", testClientID, testClientSecret},
		{"ftp://api.instamojo.com", testClientID, testClientSecret},
		{"http://api.instamojo.com", testClientID, testClientSecret},
		{"https://user:pw@api.instamojo.com", testClientID, testClientSecret},
		{"https://api.instamojo.com?x=1", testClientID, testClientSecret},
		{"api.instamojo.com", testClientID, testClientSecret},
		{"https://api.instamojo.com", " ", testClientSecret},
		{"https://api.instamojo.com", testClientID, ""},
	}
	for _, b := range bad {
		if _, err := NewClient(b.base, b.id, b.secret, time.Second); !errors.Is(err, ErrConfigInvalid) {
			t.Fatalf("%+v: got %v", b, err)
		}
	}
	if _, err := NewClient("https://api.instamojo.com", testClientID, testClientSecret, 0); !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("zero timeout: got %v", err)
	}
	for _, ok := range []string{"https://api.instamojo.com", "https://test.instamojo.com/", "http://localhost:8080", "http://127.0.0.1:1"} {
		c, err := NewClient(ok, testClientID, testClientSecret, time.Second)
		if err != nil {
			t.Fatalf("%s: %v", ok, err)
		}
		_ = c.Close()
	}
}

func TestCreatePaymentRequest_Success(t *testing.T) {
	f := newFake(t)
	c, _ := setup(t, f)
	resp, err := c.CreatePaymentRequest(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("CreatePaymentRequest: %v", err)
	}
	if resp.ID != "05f317448ad84649aa1a9c7328edb015" || !strings.HasPrefix(resp.LongURL, "https://www.instamojo.com/") || resp.Amount != "2500.00" || resp.Status != PaymentRequestStatusPending {
		t.Fatalf("response %+v", resp)
	}
	if resp.ShortURL != "" || resp.Payments == nil || len(resp.Payments) != 0 || !resp.MarkFulfilled || resp.CreatedAt.IsZero() || resp.CreatedAt.Year() != 2016 {
		t.Fatalf("response details %+v", resp)
	}
	f.mu.Lock()
	form, auth := f.lastForm, f.lastAuth
	f.mu.Unlock()
	if auth != "Bearer token-1" {
		t.Fatalf("auth header %q", auth)
	}
	want := map[string]string{
		"amount":                  "2500.00",
		"purpose":                 "Order 1",
		"buyer_name":              "John Doe",
		"email":                   "foo@example.com",
		"phone":                   "+919999999999",
		"redirect_url":            "https://shop.example.com/r/",
		"webhook":                 "https://shop.example.com/w/",
		"allow_repeated_payments": "false",
		"send_email":              "true",
	}
	for key, value := range want {
		if form.Get(key) != value {
			t.Fatalf("form %s = %q, want %q", key, form.Get(key), value)
		}
	}
	if len(form) != len(want) {
		t.Fatalf("unexpected form fields %v", form)
	}
}

func TestCreatePaymentRequest_OptionalFieldsOmitted(t *testing.T) {
	f := newFake(t)
	c, _ := setup(t, f)
	expires := time.Now().Add(10 * time.Minute)
	_, err := c.CreatePaymentRequest(context.Background(), CreatePaymentRequestRequest{Amount: 9, Purpose: "Order 2", AllowRepeatedPayments: true, ExpiresAt: expires})
	if err != nil {
		t.Fatalf("CreatePaymentRequest: %v", err)
	}
	f.mu.Lock()
	form := f.lastForm
	f.mu.Unlock()
	for _, key := range []string{"buyer_name", "email", "phone", "redirect_url", "webhook"} {
		if form.Has(key) {
			t.Fatalf("%s should be omitted", key)
		}
	}
	if form.Get("allow_repeated_payments") != "true" || form.Get("expires_at") != expires.UTC().Format(time.RFC3339) || form.Get("amount") != "9.00" {
		t.Fatalf("form %v", form)
	}
}

func TestCreatePaymentRequest_Validation(t *testing.T) {
	f := newFake(t)
	c, _ := setup(t, f)
	cases := []struct {
		name   string
		mutate func(*CreatePaymentRequestRequest)
		want   error
	}{
		{"amount low", func(r *CreatePaymentRequestRequest) { r.Amount = 8 }, ErrAmountInvalid},
		{"amount high", func(r *CreatePaymentRequestRequest) { r.Amount = 200001 }, ErrAmountInvalid},
		{"amount negative", func(r *CreatePaymentRequestRequest) { r.Amount = -1 }, ErrAmountInvalid},
		{"purpose empty", func(r *CreatePaymentRequestRequest) { r.Purpose = "   " }, ErrPurposeInvalid},
		{"purpose long", func(r *CreatePaymentRequestRequest) { r.Purpose = strings.Repeat("a", 31) }, ErrPurposeInvalid},
		{"purpose control", func(r *CreatePaymentRequestRequest) { r.Purpose = "a\x00b" }, ErrPurposeInvalid},
		{"buyer long", func(r *CreatePaymentRequestRequest) { r.BuyerName = strings.Repeat("a", 101) }, ErrBuyerNameInvalid},
		{"email bad", func(r *CreatePaymentRequestRequest) { r.Email = "nope" }, ErrEmailInvalid},
		{"email required", func(r *CreatePaymentRequestRequest) { r.Email = "" }, ErrEmailRequired},
		{"phone bad", func(r *CreatePaymentRequestRequest) { r.Phone = "12345" }, ErrPhoneInvalid},
		{"redirect relative", func(r *CreatePaymentRequestRequest) { r.RedirectURL = "/r" }, ErrRedirectURLInvalid},
		{"redirect scheme", func(r *CreatePaymentRequestRequest) { r.RedirectURL = "javascript://x" }, ErrRedirectURLInvalid},
		{"webhook bad", func(r *CreatePaymentRequestRequest) { r.WebhookURL = "ftp://x.com" }, ErrWebhookURLInvalid},
		{"webhook userinfo", func(r *CreatePaymentRequestRequest) { r.WebhookURL = "https://u:p@x.com" }, ErrWebhookURLInvalid},
		{"expired", func(r *CreatePaymentRequestRequest) { r.ExpiresAt = time.Now().Add(-time.Second) }, ErrExpiresAtInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := validRequest()
			tc.mutate(&req)
			if _, err := c.CreatePaymentRequest(context.Background(), req); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
	if f.tokenCalls.Load() != 0 || f.createCalls.Load() != 0 {
		t.Fatalf("validation errors must not hit the network")
	}
}

func TestToken_CachedAndRefreshedAfterExpiry(t *testing.T) {
	f := newFake(t)
	c, _ := setup(t, f)
	clock := time.Now()
	var clockMu sync.Mutex
	c.now = func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		return clock
	}
	for range 3 {
		if _, err := c.CreatePaymentRequest(context.Background(), validRequest()); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	if f.tokenCalls.Load() != 1 {
		t.Fatalf("token calls %d, want 1", f.tokenCalls.Load())
	}
	clockMu.Lock()
	clock = clock.Add(36000*time.Second - 59*time.Second)
	clockMu.Unlock()
	if _, err := c.CreatePaymentRequest(context.Background(), validRequest()); err != nil {
		t.Fatalf("create: %v", err)
	}
	if f.tokenCalls.Load() != 2 {
		t.Fatalf("token should refresh inside the 60s margin, calls %d", f.tokenCalls.Load())
	}
	f.mu.Lock()
	auth := f.lastAuth
	f.mu.Unlock()
	if auth != "Bearer token-2" {
		t.Fatalf("auth %q", auth)
	}
}

func TestToken_NoThunderingHerd(t *testing.T) {
	f := newFake(t)
	f.tokenDelay = 50 * time.Millisecond
	c, _ := setup(t, f)
	var wg sync.WaitGroup
	errs := make(chan error, 50)
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := c.CreatePaymentRequest(context.Background(), validRequest())
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	if f.tokenCalls.Load() != 1 || f.createCalls.Load() != 50 {
		t.Fatalf("token calls %d create calls %d", f.tokenCalls.Load(), f.createCalls.Load())
	}
}

func TestCreatePaymentRequest_RetriesOnceOn401(t *testing.T) {
	f := newFake(t)
	f.createHandler = func(w http.ResponseWriter, r *http.Request, n int64) {
		if r.Header.Get("Authorization") == "Bearer token-1" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"success":false,"message":"Authentication credentials were not provided."}`))
			return
		}
		_, _ = w.Write([]byte(successBody))
	}
	c, _ := setup(t, f)
	if _, err := c.CreatePaymentRequest(context.Background(), validRequest()); err != nil {
		t.Fatalf("create: %v", err)
	}
	if f.tokenCalls.Load() != 2 || f.createCalls.Load() != 2 {
		t.Fatalf("token %d create %d", f.tokenCalls.Load(), f.createCalls.Load())
	}
}

func TestCreatePaymentRequest_401Twice(t *testing.T) {
	f := newFake(t)
	f.createHandler = func(w http.ResponseWriter, _ *http.Request, _ int64) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"success":false,"message":"Authentication credentials were not provided."}`))
	}
	c, _ := setup(t, f)
	_, err := c.CreatePaymentRequest(context.Background(), validRequest())
	if !errors.Is(err, ErrUnauthorized) || !strings.Contains(err.Error(), "Authentication credentials") {
		t.Fatalf("got %v", err)
	}
	if f.createCalls.Load() != 2 {
		t.Fatalf("create calls %d, want 2", f.createCalls.Load())
	}
}

func TestInvalidateToken_KeepsNewerToken(t *testing.T) {
	c := &client{token: "fresh", tokenExp: time.Now().Add(time.Hour)}
	c.invalidateToken("stale")
	if c.token != "fresh" {
		t.Fatalf("newer token was wiped")
	}
	c.invalidateToken("fresh")
	if c.token != "" || !c.tokenExp.IsZero() {
		t.Fatalf("token not cleared")
	}
}

func TestCreatePaymentRequest_UpstreamErrors(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		want    error
		contain string
	}{
		{"validation", http.StatusBadRequest, `{"purpose":["This field is required."],"amount":["This field is required.","Too low."]}`, ErrValidationFailed, "amount: This field is required., Too low.; purpose: This field is required."},
		{"nested message", http.StatusBadRequest, `{"success":false,"message":{"phone":["Invalid."]}}`, ErrValidationFailed, "phone: Invalid."},
		{"server error", http.StatusBadGateway, `<html>bad gateway</html>`, ErrUpstream, "status 502"},
		{"forbidden", http.StatusForbidden, `{"success":false,"message":"nope"}`, ErrForbidden, "message: nope"},
		{"bad json", http.StatusOK, `{"id":`, ErrResponseInvalid, ""},
		{"missing longurl", http.StatusOK, `{"id":"abc"}`, ErrResponseInvalid, "longurl"},
		{"unknown status", http.StatusOK, `{"id":"abc","longurl":"https://x.com/a","status":"Weird"}`, ErrResponseInvalid, "Weird"},
		{"missing status", http.StatusOK, `{"id":"abc","longurl":"https://x.com/a"}`, ErrResponseInvalid, "unknown status"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFake(t)
			f.createHandler = func(w http.ResponseWriter, _ *http.Request, _ int64) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}
			c, _ := setup(t, f)
			_, err := c.CreatePaymentRequest(context.Background(), validRequest())
			if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), tc.contain) {
				t.Fatalf("got %v, want %v containing %q", err, tc.want, tc.contain)
			}
		})
	}
}

func TestToken_Errors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"unsupported grant", http.StatusBadRequest, `{"error":"unsupported_grant_type"}`, ErrAuthFailed},
		{"server error", http.StatusInternalServerError, `oops`, ErrUpstream},
		{"bad json", http.StatusOK, `nope`, ErrResponseInvalid},
		{"no token", http.StatusOK, `{"access_token":"","expires_in":10}`, ErrResponseInvalid},
		{"no expiry", http.StatusOK, `{"access_token":"x","expires_in":0}`, ErrResponseInvalid},
		{"wrong type", http.StatusOK, `{"access_token":"x","token_type":"mac","expires_in":10}`, ErrResponseInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFake(t)
			f.tokenStatus = tc.status
			f.tokenBody = func(int64) string { return tc.body }
			c, _ := setup(t, f)
			if _, err := c.CreatePaymentRequest(context.Background(), validRequest()); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			if f.createCalls.Load() != 0 {
				t.Fatalf("create must not be called without a token")
			}
		})
	}
}

func TestToken_WrongCredentials(t *testing.T) {
	f := newFake(t)
	server := httptest.NewServer(f)
	t.Cleanup(server.Close)
	c, err := NewClient(server.URL, testClientID, "wrong", time.Second)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer c.Close()
	_, err = c.CreatePaymentRequest(context.Background(), validRequest())
	if !errors.Is(err, ErrAuthFailed) || !strings.Contains(err.Error(), "invalid_client") || strings.Contains(err.Error(), "wrong") {
		t.Fatalf("got %v", err)
	}
}

func TestPaymentRequestStatus(t *testing.T) {
	for _, s := range []PaymentRequestStatus{PaymentRequestStatusPending, PaymentRequestStatusSent, PaymentRequestStatusCompleted, PaymentRequestStatusFailed, PaymentRequestStatusExpired} {
		if !s.IsValid() || s.String() != string(s) {
			t.Fatalf("%s should be valid", s)
		}
	}
	for _, s := range []PaymentRequestStatus{"", "pending", "Weird"} {
		if s.IsValid() {
			t.Fatalf("%q should be invalid", s)
		}
	}
	if PaymentRequestStatusPending.IsTerminal() || PaymentRequestStatusSent.IsTerminal() || !PaymentRequestStatusCompleted.IsTerminal() || !PaymentRequestStatusFailed.IsTerminal() || !PaymentRequestStatusExpired.IsTerminal() {
		t.Fatalf("terminal states wrong")
	}
}

func TestTokenLifetime(t *testing.T) {
	cases := map[int64]time.Duration{
		36000: 36000*time.Second - time.Minute,
		60:    30 * time.Second,
		1:     500 * time.Millisecond,
	}
	for in, want := range cases {
		if got := tokenLifetime(in); got != want {
			t.Fatalf("%d: got %v want %v", in, got, want)
		}
	}
}

func TestCreatePaymentRequest_ContextAndClose(t *testing.T) {
	f := newFake(t)
	c, server := setup(t, f)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.CreatePaymentRequest(ctx, validRequest()); !errors.Is(err, ErrCtxInvalid) {
		t.Fatalf("cancelled: got %v", err)
	}
	if _, err := c.CreatePaymentRequest(nil, validRequest()); !errors.Is(err, ErrCtxInvalid) {
		t.Fatalf("nil ctx: got %v", err)
	}
	f.tokenDelay = 200 * time.Millisecond
	timeoutCtx, cancelTimeout := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelTimeout()
	if _, err := c.CreatePaymentRequest(timeoutCtx, validRequest()); !errors.Is(err, ErrCtxInvalid) {
		t.Fatalf("timeout: got %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
	if _, err := c.CreatePaymentRequest(context.Background(), validRequest()); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed: got %v", err)
	}
	server.Close()
}

func TestCreatePaymentRequest_NetworkError(t *testing.T) {
	f := newFake(t)
	c, server := setup(t, f)
	server.Close()
	if _, err := c.CreatePaymentRequest(context.Background(), validRequest()); !errors.Is(err, ErrUpstream) {
		t.Fatalf("got %v", err)
	}
}

func TestDescribeErrorBody_Truncates(t *testing.T) {
	got := describeErrorBody(strings.Repeat("x", 2000))
	if len(got) != maxErrorBodyLength+3 || !strings.HasSuffix(got, "...") {
		t.Fatalf("len %d", len(got))
	}
	if describeErrorBody(`{"success":false}`) != `{"success":false}` {
		t.Fatalf("empty error map should fall back to body")
	}
	if got := describeErrorBody(`{"detail":{"b":1,"a":["x"]}}`); got != "detail: a: x, b: 1" {
		t.Fatalf("got %q", got)
	}
}

func TestSandboxSmoke(t *testing.T) {
	if os.Getenv("INSTAMOJO_SMOKE") != "1" {
		t.Skip("set INSTAMOJO_SMOKE=1 with INSTAMOJO_CLIENT_ID and INSTAMOJO_CLIENT_SECRET to hit the sandbox")
	}
	c, err := NewClient("https://test.instamojo.com", os.Getenv("INSTAMOJO_CLIENT_ID"), os.Getenv("INSTAMOJO_CLIENT_SECRET"), 15*time.Second)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer c.Close()
	resp, err := c.CreatePaymentRequest(context.Background(), CreatePaymentRequestRequest{Amount: 10, Purpose: "Smoke test", BuyerName: "Smoke Test"})
	if err != nil {
		t.Fatalf("CreatePaymentRequest: %v", err)
	}
	t.Logf("created %s status %s", resp.ID, resp.Status)
	fetched, err := c.GetPaymentRequest(context.Background(), resp.ID)
	if err != nil || fetched.ID != resp.ID {
		t.Fatalf("GetPaymentRequest: %v", err)
	}
}

const (
	testRequestID        = "87b0425b6baf4012a3abd1b6f4644741"
	testPaymentID        = "MOJO3207U05Q80983266"
	completedRequest     = `{"id":"87b0425b6baf4012a3abd1b6f4644741","user":"https://api.instamojo.com/v2/users/60d/","phone":"+919898989898","email":"rahul@example.com","buyer_name":"Rahul","amount":"10.00","purpose":"Fifa 10","status":"Completed","payments":["https://api.instamojo.com/v2/payments/MOJO3207K05Q80978898/"],"send_sms":false,"send_email":false,"sms_status":null,"email_status":null,"shorturl":"https://imjo.in/bdEqE5","longurl":"https://www.instamojo.com/@rahul/87b0425b6baf4012a3abd1b6f4644741","redirect_url":"https://www.google.com/","webhook":"https://webhook.site/x","scheduled_at":null,"expires_at":"2023-02-07T07:00:00Z","allow_repeated_payments":false,"mark_fulfilled":false,"created_at":"2023-02-07T06:49:07.209707Z","modified_at":"2023-02-07T06:50:50.349715Z","resource_uri":"https://api.instamojo.com/v2/payment_requests/87b0425b6baf4012a3abd1b6f4644741/"}`
	successfulPayment    = `{"id":"MOJO3207U05Q80983266","title":"FIFA 10","payment_type":"api_rap","payment_request":"https://api.instamojo.com/v2/payment_requests/b318750f06e3491484acef4cb4ff7966/","status":true,"link":null,"product":null,"seller":"https://api.instamojo.com/v2/users/60d/","currency":"INR","amount":"10.00","name":"Rahul","email":"rahul@example.com","phone":"+919898989898","payout":null,"fees":"3.20","total_taxes":"0.58","cases":[],"affiliate_id":null,"affiliate_commission":"0.00","order_info":{"shipping_address":null,"shipping_city":null,"shipping_state":null,"shipping_zip":null,"shipping_country":null,"quantity":1,"unit_price":"10.00","custom_fields":{},"variants":[]},"instrument_type":"QR","billing_instrument":"Order based QR Code","failure":null,"created_at":"2023-02-07T08:30:33.902672Z","updated_at":"2023-02-07T08:30:34.300356Z","tax_invoice_id":"","resource_uri":"https://api.instamojo.com/v2/payments/MOJO3207U05Q80983266/"}`
	failedPaymentBody    = `{"id":"MOJO3209V05A98786622","status":false,"currency":"INR","amount":"10.00","fees":null,"total_taxes":null,"affiliate_commission":null,"order_info":{"quantity":1,"unit_price":"10.00","custom_fields":{},"variants":[]},"instrument_type":"CARD","billing_instrument":"International Regular Credit Card (Visa/Mastercard)","failure":{"reason":"AUTHORIZATION_FAILED","message":"Transaction was declined by the bank"},"created_at":"2023-02-09T07:35:56.590046Z","updated_at":"2023-02-09T07:36:04.285084Z"}`
	initiatedPaymentBody = `{"id":"MOJO3209V05A98786623","status":null,"currency":"INR","amount":"10.00","instrument_type":"UPI","failure":null,"created_at":"2023-02-09T07:35:56Z","updated_at":"2023-02-09T07:35:56Z"}`
)

func fixedGet(body string, status int) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestGetPaymentRequest(t *testing.T) {
	f := newFake(t)
	f.getHandler = fixedGet(completedRequest, http.StatusOK)
	c, _ := setup(t, f)
	resp, err := c.GetPaymentRequest(context.Background(), " "+testRequestID+" ")
	if err != nil {
		t.Fatalf("GetPaymentRequest: %v", err)
	}
	f.mu.Lock()
	path, auth := f.lastPath, f.lastAuth
	f.mu.Unlock()
	if path != paymentRequestPath+testRequestID+"/" || auth != "Bearer token-1" {
		t.Fatalf("path %q auth %q", path, auth)
	}
	if resp.ID != testRequestID || resp.Status != PaymentRequestStatusCompleted || len(resp.Payments) != 1 || !strings.HasSuffix(resp.Payments[0], "/MOJO3207K05Q80978898/") {
		t.Fatalf("response %+v", resp)
	}
	if resp.ScheduledAt != nil || resp.ExpiresAt == nil || resp.ExpiresAt.Minute() != 0 || resp.SMSStatus != "" || resp.ShortURL != "https://imjo.in/bdEqE5" {
		t.Fatalf("nullable fields %+v", resp)
	}
}

func TestGetPaymentRequest_Errors(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		status int
		want   error
	}{
		{"not found", `{"success":false,"message":"Not found."}`, http.StatusNotFound, ErrNotFound},
		{"id mismatch", strings.Replace(completedRequest, testRequestID+`","user"`, `aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","user"`, 1), http.StatusOK, ErrResponseInvalid},
		{"server error", `boom`, http.StatusInternalServerError, ErrUpstream},
		{"bad payments shape", strings.Replace(completedRequest, `["https://api.instamojo.com/v2/payments/MOJO3207K05Q80978898/"]`, `[{"x":1}]`, 1), http.StatusOK, ErrResponseInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFake(t)
			f.getHandler = fixedGet(tc.body, tc.status)
			c, _ := setup(t, f)
			if _, err := c.GetPaymentRequest(context.Background(), testRequestID); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestGetPaymentDetails(t *testing.T) {
	f := newFake(t)
	f.getHandler = fixedGet(successfulPayment, http.StatusOK)
	c, _ := setup(t, f)
	p, err := c.GetPaymentDetails(context.Background(), testPaymentID)
	if err != nil {
		t.Fatalf("GetPaymentDetails: %v", err)
	}
	f.mu.Lock()
	path := f.lastPath
	f.mu.Unlock()
	if path != paymentPath+testPaymentID+"/" {
		t.Fatalf("path %q", path)
	}
	if p.PaymentStatus() != PaymentStatusSuccessful || p.Status == nil || !*p.Status || p.Amount != "10.00" || p.Fees != "3.20" || p.Currency != "INR" {
		t.Fatalf("payment %+v", p)
	}
	if p.InstrumentType != InstrumentTypeQR || !p.InstrumentType.IsKnown() || p.Failure != nil || p.Payout != "" || p.OrderInfo.Quantity != 1 || p.OrderInfo.UnitPrice != "10.00" || p.Cases == nil {
		t.Fatalf("payment details %+v", p)
	}
	if !strings.HasSuffix(p.PaymentRequest, "/b318750f06e3491484acef4cb4ff7966/") || p.UpdatedAt.IsZero() {
		t.Fatalf("payment request link %q", p.PaymentRequest)
	}
}

func TestGetPaymentDetails_FailedAndInitiated(t *testing.T) {
	f := newFake(t)
	f.getHandler = fixedGet(failedPaymentBody, http.StatusOK)
	c, _ := setup(t, f)
	p, err := c.GetPaymentDetails(context.Background(), "MOJO3209V05A98786622")
	if err != nil {
		t.Fatalf("failed payment: %v", err)
	}
	if p.PaymentStatus() != PaymentStatusFailed || p.Failure == nil || p.Failure.Reason != "AUTHORIZATION_FAILED" || p.Fees != "" || p.InstrumentType != InstrumentTypeCard {
		t.Fatalf("failed payment %+v", p)
	}
	f.getHandler = fixedGet(initiatedPaymentBody, http.StatusOK)
	p, err = c.GetPaymentDetails(context.Background(), "MOJO3209V05A98786623")
	if err != nil {
		t.Fatalf("initiated payment: %v", err)
	}
	if p.PaymentStatus() != PaymentStatusInitiated || p.Status != nil || p.PaymentStatus().IsTerminal() {
		t.Fatalf("initiated payment %+v", p)
	}
}

func TestGetPaymentDetails_Errors(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		status int
		want   error
	}{
		{"not found", `{"success":false,"message":"Not found."}`, http.StatusNotFound, ErrNotFound},
		{"id mismatch", successfulPayment, http.StatusOK, ErrResponseInvalid},
		{"no amount", `{"id":"MOJO3207U05Q80983267","status":true}`, http.StatusOK, ErrResponseInvalid},
		{"bad json", `{`, http.StatusOK, ErrResponseInvalid},
		{"forbidden", `{"success":false,"message":"nope"}`, http.StatusForbidden, ErrForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFake(t)
			f.getHandler = fixedGet(tc.body, tc.status)
			c, _ := setup(t, f)
			if _, err := c.GetPaymentDetails(context.Background(), "MOJO3207U05Q80983267"); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestGet_RetriesOnceOn401(t *testing.T) {
	f := newFake(t)
	f.getHandler = func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer token-1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(successfulPayment))
	}
	c, _ := setup(t, f)
	if _, err := c.GetPaymentDetails(context.Background(), testPaymentID); err != nil {
		t.Fatalf("GetPaymentDetails: %v", err)
	}
	if f.tokenCalls.Load() != 2 || f.getCalls.Load() != 2 {
		t.Fatalf("token %d get %d", f.tokenCalls.Load(), f.getCalls.Load())
	}
}

func TestGet_InvalidIDsNeverHitNetwork(t *testing.T) {
	f := newFake(t)
	c, _ := setup(t, f)
	for _, id := range []string{"", "87B0425B6BAF4012A3ABD1B6F4644741", "87b0425b6baf4012a3abd1b6f464474", "../payments/x", testRequestID + "/../x"} {
		if _, err := c.GetPaymentRequest(context.Background(), id); !errors.Is(err, ErrPaymentRequestIDInvalid) {
			t.Fatalf("request id %q: got %v", id, err)
		}
	}
	for _, id := range []string{"", "MOJO", "mojo3207U05Q80983266", "MOJO3207U05Q8098/../x", "MOJO3207U05Q80983266?x=1"} {
		if _, err := c.GetPaymentDetails(context.Background(), id); !errors.Is(err, ErrPaymentIDInvalid) {
			t.Fatalf("payment id %q: got %v", id, err)
		}
	}
	if f.tokenCalls.Load() != 0 || f.getCalls.Load() != 0 {
		t.Fatalf("invalid ids must not hit the network")
	}
}

func TestGet_ContextAndClosed(t *testing.T) {
	f := newFake(t)
	c, _ := setup(t, f)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.GetPaymentRequest(ctx, testRequestID); !errors.Is(err, ErrCtxInvalid) {
		t.Fatalf("got %v", err)
	}
	if _, err := c.GetPaymentDetails(nil, testPaymentID); !errors.Is(err, ErrCtxInvalid) {
		t.Fatalf("got %v", err)
	}
	_ = c.Close()
	if _, err := c.GetPaymentRequest(context.Background(), testRequestID); !errors.Is(err, ErrClosed) {
		t.Fatalf("got %v", err)
	}
	if _, err := c.GetPaymentDetails(context.Background(), testPaymentID); !errors.Is(err, ErrClosed) {
		t.Fatalf("got %v", err)
	}
}

func TestPaymentStatusEnum(t *testing.T) {
	for _, s := range []PaymentStatus{PaymentStatusInitiated, PaymentStatusSuccessful, PaymentStatusFailed} {
		if !s.IsValid() || s.String() != string(s) {
			t.Fatalf("%s should be valid", s)
		}
	}
	if PaymentStatus("x").IsValid() || InstrumentType("CRYPTO").IsKnown() || InstrumentTypeUPI.String() != "UPI" {
		t.Fatalf("enum checks wrong")
	}
	if !PaymentStatusSuccessful.IsTerminal() || !PaymentStatusFailed.IsTerminal() {
		t.Fatalf("terminal wrong")
	}
}
