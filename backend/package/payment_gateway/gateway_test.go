package paymentGateway

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	instamojoClient "github.com/thesahibnanda-max/eCommerce/backend/package/clients/instamojo"
	paymentsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/payments"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const testSalt = "test-salt"

var jpegProof = append([]byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}, bytes.Repeat([]byte{0x01}, 64)...)

type fakeInstamojo struct {
	mu              sync.Mutex
	nextID          int
	createErr       error
	requests        map[string][]string
	payments        map[string]instamojoClient.Payment
	lastCreate      instamojoClient.CreatePaymentRequestRequest
	createCalls     atomic.Int64
	getRequestCalls atomic.Int64
	getPaymentCalls atomic.Int64
}

func newFakeInstamojo() *fakeInstamojo {
	return &fakeInstamojo{requests: map[string][]string{}, payments: map[string]instamojoClient.Payment{}}
}

func (f *fakeInstamojo) CreatePaymentRequest(_ context.Context, req instamojoClient.CreatePaymentRequestRequest) (instamojoClient.PaymentRequest, error) {
	f.createCalls.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastCreate = req
	if f.createErr != nil {
		return instamojoClient.PaymentRequest{}, f.createErr
	}
	f.nextID++
	id := fmt.Sprintf("%032x", f.nextID)
	f.requests[id] = nil
	return instamojoClient.PaymentRequest{ID: id, Status: instamojoClient.PaymentRequestStatusPending, LongURL: "https://www.instamojo.com/@shop/" + id, Payments: []string{}}, nil
}

func (f *fakeInstamojo) GetPaymentRequest(_ context.Context, id string) (instamojoClient.PaymentRequest, error) {
	f.getRequestCalls.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	ids, ok := f.requests[id]
	if !ok {
		return instamojoClient.PaymentRequest{}, instamojoClient.ErrNotFound
	}
	links := make([]string, 0, len(ids))
	for _, paymentID := range ids {
		links = append(links, "https://api.instamojo.com/v2/payments/"+paymentID+"/")
	}
	return instamojoClient.PaymentRequest{ID: id, Status: instamojoClient.PaymentRequestStatusPending, Payments: links}, nil
}

func (f *fakeInstamojo) GetPaymentDetails(_ context.Context, id string) (instamojoClient.Payment, error) {
	f.getPaymentCalls.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	payment, ok := f.payments[id]
	if !ok {
		return instamojoClient.Payment{}, instamojoClient.ErrNotFound
	}
	return payment, nil
}

func (f *fakeInstamojo) Close() error {
	return nil
}

func (f *fakeInstamojo) pay(requestID, paymentID string, status *bool, amount string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests[requestID] = append(f.requests[requestID], paymentID)
	payment := instamojoClient.Payment{
		ID:             paymentID,
		PaymentRequest: "https://api.instamojo.com/v2/payment_requests/" + requestID + "/",
		Status:         status,
		Currency:       "INR",
		Amount:         amount,
		InstrumentType: instamojoClient.InstrumentTypeUPI,
	}
	if status != nil && !*status {
		payment.Failure = &instamojoClient.PaymentFailure{Reason: "AUTHORIZATION_FAILED", Message: "Transaction was declined by the bank"}
	}
	f.payments[paymentID] = payment
}

type fakeUPIClient struct{}

func (fakeUPIClient) GetUPIQR() string         { return "QUJD" }
func (fakeUPIClient) GetUPIQRDataURI() string  { return "data:image/jpeg;base64,QUJD" }
func (fakeUPIClient) GetUPIQRMIMEType() string { return "image/jpeg" }
func (fakeUPIClient) GetUPIID() string         { return "shop@ybl" }

type finalizedRecorder struct {
	mu     sync.Mutex
	states []State
}

func (r *finalizedRecorder) hook(_ context.Context, payment Payment) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.states = append(r.states, payment.State)
}

func (r *finalizedRecorder) snapshot() []State {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]State(nil), r.states...)
}

func boolPtr(value bool) *bool {
	return &value
}

func newPayments(t *testing.T) (paymentsRepository.Repository, string) {
	t.Helper()
	dir := t.TempDir()
	repo, err := paymentsRepository.NewRepository([]string{dir, "payments.db"}, 0o600)
	if err != nil {
		t.Fatalf("payments repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo, dir
}

func instamojoParams(t *testing.T, fake *fakeInstamojo, repo paymentsRepository.Repository, dir string, recorder *finalizedRecorder) InstamojoConstructorParams {
	t.Helper()
	return InstamojoConstructorParams{
		Client:          fake,
		Payments:        repo,
		PrivateSalt:     testSalt,
		QueuePath:       []string{dir, "queue.db"},
		QueuePermission: 0o600,
		PollInterval:    20 * time.Millisecond,
		PollWindow:      600 * time.Millisecond,
		LeaseTimeout:    500 * time.Millisecond,
		Concurrency:     4,
		RedirectURL:     "https://shop.example.com/payments/redirect",
		WebhookURL:      "https://shop.example.com/payments/webhook",
		OnFinalized:     recorder.hook,
	}
}

func newInstamojo(t *testing.T, mutate func(*InstamojoConstructorParams)) (*instamojoGateway, *fakeInstamojo, paymentsRepository.Repository, *finalizedRecorder) {
	t.Helper()
	fake := newFakeInstamojo()
	repo, dir := newPayments(t)
	recorder := &finalizedRecorder{}
	params := instamojoParams(t, fake, repo, dir, recorder)
	if mutate != nil {
		mutate(&params)
	}
	gateway, err := NewGateway(params)
	if err != nil {
		t.Fatalf("NewGateway: %v", err)
	}
	t.Cleanup(func() { _ = gateway.Close() })
	return gateway.(*instamojoGateway), fake, repo, recorder
}

func initiateRequest(orderID string) InitiateRequest {
	return InitiateRequest{OrderID: orderID, UserID: "user-1", Amount: 1499, Purpose: "Order " + orderID, BuyerName: "Test Buyer", Email: "buyer@example.com", Phone: "9999999999"}
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func stateIs(t *testing.T, g Gateway, id string, want State) func() bool {
	return func() bool {
		payment, err := g.GetPayment(context.Background(), id)
		if err != nil {
			t.Fatalf("GetPayment: %v", err)
		}
		return payment.State == want
	}
}

func signedForm(fields map[string]string, salt string) url.Values {
	form := url.Values{}
	for key, value := range fields {
		form.Set(key, value)
	}
	form.Set("mac", instamojoMAC(fields, salt))
	return form
}

func hasEvent(model paymentsRepository.Model, kind string) bool {
	for _, event := range model.Events {
		if event.Kind == kind {
			return true
		}
	}
	return false
}

func TestNewGateway_Params(t *testing.T) {
	fake := newFakeInstamojo()
	repo, dir := newPayments(t)
	base := instamojoParams(t, fake, repo, dir, &finalizedRecorder{})
	bad := []func(*InstamojoConstructorParams){
		func(p *InstamojoConstructorParams) { p.Client = nil },
		func(p *InstamojoConstructorParams) { p.Payments = nil },
		func(p *InstamojoConstructorParams) { p.PrivateSalt = " " },
		func(p *InstamojoConstructorParams) { p.QueuePath = nil },
		func(p *InstamojoConstructorParams) { p.QueuePermission = 0 },
		func(p *InstamojoConstructorParams) { p.PollInterval = 0 },
		func(p *InstamojoConstructorParams) { p.PollWindow = p.PollInterval / 2 },
		func(p *InstamojoConstructorParams) { p.LeaseTimeout = 0 },
		func(p *InstamojoConstructorParams) { p.Concurrency = 0 },
		func(p *InstamojoConstructorParams) { p.RedirectURL = "ftp://x" },
		func(p *InstamojoConstructorParams) { p.WebhookURL = "/relative" },
	}
	for i, mutate := range bad {
		params := base
		mutate(&params)
		if _, err := NewGateway(params); !errors.Is(err, ErrParamsInvalid) {
			t.Fatalf("instamojo case %d: %v", i, err)
		}
	}
	upiBad := []UPIConstructorParams{
		{Client: nil, Payments: repo, PayeeName: "Shop"},
		{Client: fakeUPIClient{}, Payments: nil, PayeeName: "Shop"},
		{Client: fakeUPIClient{}, Payments: repo, PayeeName: "  "},
		{Client: fakeUPIClient{}, Payments: repo, PayeeName: strings.Repeat("a", 100)},
	}
	for i, params := range upiBad {
		if _, err := NewGateway(params); !errors.Is(err, ErrParamsInvalid) {
			t.Fatalf("upi case %d: %v", i, err)
		}
	}
	var nilInstamojo *InstamojoConstructorParams
	var nilUPI *UPIConstructorParams
	for _, params := range []ConstructorParams{nil, nilInstamojo, nilUPI} {
		if _, err := NewGateway(params); !errors.Is(err, ErrParamsInvalid) {
			t.Fatalf("nil params %T: %v", params, err)
		}
	}
	pointer := base
	pointer.QueuePath = []string{dir, "queue-pointer.db"}
	g, err := NewGateway(&pointer)
	if err != nil || g.Provider() != ProviderInstamojo {
		t.Fatalf("pointer params: %v", err)
	}
	_ = g.Close()
	u, err := NewGateway(&UPIConstructorParams{Client: fakeUPIClient{}, Payments: repo, PayeeName: "Shop"})
	if err != nil || u.Provider() != ProviderSelfUPI {
		t.Fatalf("upi pointer params: %v", err)
	}
	_ = u.Close()
	if _, err := NewGateway(InstamojoConstructorParams{Client: fake, Payments: repo, PrivateSalt: "s", QueuePath: []string{dir, "missing", "q.db"}, QueuePermission: 0o600, PollInterval: time.Second, PollWindow: time.Second, LeaseTimeout: time.Second, Concurrency: 1}); err == nil {
		t.Fatalf("unwritable queue path must fail")
	}
}

func TestInstamojo_InitiateRedirectsAndIsIdempotent(t *testing.T) {
	g, fake, _, _ := newInstamojo(t, func(p *InstamojoConstructorParams) { p.PollInterval, p.PollWindow = time.Hour, 2*time.Hour })
	before := time.Now()
	resp, err := g.InitiatePayment(context.Background(), initiateRequest("order-1"))
	if err != nil {
		t.Fatalf("Initiate: %v", err)
	}
	p := resp.Payment
	if resp.Action.Type != ActionRedirect || resp.Action.RedirectURL == "" || resp.Action.RedirectURL != p.CheckoutURL || p.State != StateAwaitingPayment || p.GatewayReference == "" || p.Provider != ProviderInstamojo {
		t.Fatalf("response %+v", resp)
	}
	fake.mu.Lock()
	created := fake.lastCreate
	fake.mu.Unlock()
	if created.Purpose != p.ID || created.Amount != 1499 || created.Email != "buyer@example.com" || created.WebhookURL != "https://shop.example.com/payments/webhook" || created.RedirectURL != "https://shop.example.com/payments/redirect" {
		t.Fatalf("create request %+v", created)
	}
	if created.ExpiresAt.Before(before.Add(2*time.Hour-time.Minute)) || created.ExpiresAt.After(time.Now().Add(2*time.Hour)) {
		t.Fatalf("expires at %v", created.ExpiresAt)
	}
	again, err := g.InitiatePayment(context.Background(), initiateRequest("order-1"))
	if err != nil || again.Payment.ID != p.ID || again.Action.RedirectURL != resp.Action.RedirectURL || fake.createCalls.Load() != 1 {
		t.Fatalf("idempotent initiate: %v calls=%d", err, fake.createCalls.Load())
	}
	other := initiateRequest("order-1")
	other.Amount = 2000
	if _, err := g.InitiatePayment(context.Background(), other); !errors.Is(err, paymentsRepository.ErrOpenPaymentConflict) {
		t.Fatalf("conflict: %v", err)
	}
}

func TestInstamojo_InitiateValidation(t *testing.T) {
	g, fake, _, _ := newInstamojo(t, nil)
	cases := []struct {
		mutate func(*InitiateRequest)
		want   error
	}{
		{func(r *InitiateRequest) { r.OrderID = "" }, ErrOrderIDInvalid},
		{func(r *InitiateRequest) { r.UserID = "a\x00b" }, ErrUserIDInvalid},
		{func(r *InitiateRequest) { r.Amount = 8 }, ErrAmountInvalid},
		{func(r *InitiateRequest) { r.Amount = 200001 }, ErrAmountInvalid},
		{func(r *InitiateRequest) { r.Purpose = strings.Repeat("p", 101) }, ErrPurposeInvalid},
	}
	for i, tc := range cases {
		req := initiateRequest("order-v")
		tc.mutate(&req)
		if _, err := g.InitiatePayment(context.Background(), req); !errors.Is(err, tc.want) {
			t.Fatalf("case %d: got %v, want %v", i, err, tc.want)
		}
	}
	if fake.createCalls.Load() != 0 {
		t.Fatalf("invalid requests must not reach instamojo")
	}
}

func TestInstamojo_CreateFailureMarksFailedAndAllowsRetry(t *testing.T) {
	g, fake, repo, recorder := newInstamojo(t, nil)
	fake.createErr = instamojoClient.ErrUpstream
	_, err := g.InitiatePayment(context.Background(), initiateRequest("order-1"))
	if !errors.Is(err, ErrProviderFailed) || !errors.Is(err, instamojoClient.ErrUpstream) {
		t.Fatalf("got %v", err)
	}
	list, err := repo.ListOrderPayments(context.Background(), "order-1")
	if err != nil || len(list) != 1 || list[0].State != StateFailed || list[0].FailureReason == "" {
		t.Fatalf("payment after failure %v %+v", err, list)
	}
	if got := recorder.snapshot(); len(got) != 1 || got[0] != StateFailed {
		t.Fatalf("finalized %v", got)
	}
	fake.mu.Lock()
	fake.createErr = nil
	fake.mu.Unlock()
	resp, err := g.InitiatePayment(context.Background(), initiateRequest("order-1"))
	if err != nil || resp.Payment.ID == list[0].ID {
		t.Fatalf("retry after failure: %v", err)
	}
}

func TestInstamojo_CreateFailureKeepsProviderErrorOutOfCustomerReason(t *testing.T) {
	g, fake, repo, _ := newInstamojo(t, nil)
	fake.createErr = fmt.Errorf("%w: status 403: message: Payment Requests are not enabled for your account.", instamojoClient.ErrForbidden)
	if _, err := g.InitiatePayment(context.Background(), initiateRequest("order-1")); !errors.Is(err, instamojoClient.ErrForbidden) {
		t.Fatalf("got %v", err)
	}
	list, err := repo.ListOrderPayments(context.Background(), "order-1")
	if err != nil || len(list) != 1 {
		t.Fatalf("payments %v %+v", err, list)
	}
	if list[0].FailureReason != reasonPaymentSetupFailed || strings.Contains(list[0].FailureReason, "instamojo") {
		t.Fatalf("customer-facing reason = %q", list[0].FailureReason)
	}
	model, err := repo.GetPayment(context.Background(), list[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range model.Events {
		if detail, _ := event.Data["error"].(string); event.Kind == "setup_failed" && strings.Contains(detail, "Payment Requests are not enabled") {
			found = true
		}
	}
	if !found {
		t.Fatalf("provider error not kept in admin event history: %+v", model.Events)
	}
}

func TestInstamojo_PollReachesSuccessOnce(t *testing.T) {
	g, fake, repo, recorder := newInstamojo(t, func(p *InstamojoConstructorParams) { p.PollWindow = 5 * time.Second })
	resp, err := g.InitiatePayment(context.Background(), initiateRequest("order-1"))
	if err != nil {
		t.Fatalf("Initiate: %v", err)
	}
	fake.pay(resp.Payment.GatewayReference, "MOJO5a06005J21512197", boolPtr(true), "1499.00")
	eventually(t, "success", stateIs(t, g, resp.Payment.ID, StateSucceeded))
	time.Sleep(100 * time.Millisecond)
	model, err := repo.GetPayment(context.Background(), resp.Payment.ID)
	if err != nil || model.GatewayPaymentID != "MOJO5a06005J21512197" || !hasEvent(model, "payment_succeeded") {
		t.Fatalf("model %v %+v", err, model)
	}
	if got := recorder.snapshot(); len(got) != 1 || got[0] != StateSucceeded {
		t.Fatalf("finalized %v", got)
	}
}

func TestInstamojo_FailedAttemptThenSuccess(t *testing.T) {
	g, fake, repo, recorder := newInstamojo(t, func(p *InstamojoConstructorParams) { p.PollWindow = 5 * time.Second })
	resp, err := g.InitiatePayment(context.Background(), initiateRequest("order-1"))
	if err != nil {
		t.Fatalf("Initiate: %v", err)
	}
	fake.pay(resp.Payment.GatewayReference, "MOJOFAILED0000000001", boolPtr(false), "1499.00")
	eventually(t, "failure recorded", func() bool {
		model, err := repo.GetPayment(context.Background(), resp.Payment.ID)
		return err == nil && model.FailureReason != "" && hasEvent(model, "payment_failed")
	})
	if payment, _ := g.GetPayment(context.Background(), resp.Payment.ID); payment.State != StateAwaitingPayment {
		t.Fatalf("a failed attempt must not close the payment, got %s", payment.State)
	}
	fake.pay(resp.Payment.GatewayReference, "MOJOSUCCESS000000001", boolPtr(true), "1499.00")
	eventually(t, "success", stateIs(t, g, resp.Payment.ID, StateSucceeded))
	payment, _ := g.GetPayment(context.Background(), resp.Payment.ID)
	if payment.FailureReason != "" || payment.GatewayPaymentID != "MOJOSUCCESS000000001" {
		t.Fatalf("payment %+v", payment)
	}
	if got := recorder.snapshot(); len(got) != 1 || got[0] != StateSucceeded {
		t.Fatalf("finalized %v", got)
	}
}

func TestInstamojo_WindowCloses(t *testing.T) {
	t.Run("expired", func(t *testing.T) {
		g, _, _, recorder := newInstamojo(t, nil)
		resp, err := g.InitiatePayment(context.Background(), initiateRequest("order-1"))
		if err != nil {
			t.Fatalf("Initiate: %v", err)
		}
		eventually(t, "expiry", stateIs(t, g, resp.Payment.ID, StateExpired))
		time.Sleep(100 * time.Millisecond)
		if got := recorder.snapshot(); len(got) != 1 || got[0] != StateExpired {
			t.Fatalf("finalized %v", got)
		}
	})
	t.Run("failed", func(t *testing.T) {
		g, fake, _, _ := newInstamojo(t, nil)
		resp, err := g.InitiatePayment(context.Background(), initiateRequest("order-1"))
		if err != nil {
			t.Fatalf("Initiate: %v", err)
		}
		fake.pay(resp.Payment.GatewayReference, "MOJOFAILED0000000001", boolPtr(false), "1499.00")
		eventually(t, "failed", stateIs(t, g, resp.Payment.ID, StateFailed))
	})
	t.Run("amount mismatch never succeeds", func(t *testing.T) {
		g, fake, repo, _ := newInstamojo(t, nil)
		resp, err := g.InitiatePayment(context.Background(), initiateRequest("order-1"))
		if err != nil {
			t.Fatalf("Initiate: %v", err)
		}
		fake.pay(resp.Payment.GatewayReference, "MOJOCHEAP00000000001", boolPtr(true), "1.00")
		eventually(t, "expiry", stateIs(t, g, resp.Payment.ID, StateExpired))
		model, _ := repo.GetPayment(context.Background(), resp.Payment.ID)
		if !hasEvent(model, "payment_mismatch") {
			t.Fatalf("mismatch must be recorded")
		}
	})
}

func TestInstamojo_Webhook(t *testing.T) {
	g, fake, repo, recorder := newInstamojo(t, func(p *InstamojoConstructorParams) { p.PollInterval, p.PollWindow = time.Hour, 2*time.Hour })
	resp, err := g.InitiatePayment(context.Background(), initiateRequest("order-1"))
	if err != nil {
		t.Fatalf("Initiate: %v", err)
	}
	fields := map[string]string{
		"amount":             "1499.00",
		"buyer":              "buyer@example.com",
		"buyer_name":         "Test Buyer",
		"currency":           "INR",
		"payment_id":         "MOJO5a06005J21512197",
		"payment_request_id": resp.Payment.GatewayReference,
		"purpose":            resp.Payment.ID,
		"status":             "Credit",
	}
	if _, err := g.HandleWebhook(context.Background(), WebhookRequest{Form: signedForm(fields, "wrong-salt")}); !errors.Is(err, ErrWebhookSignatureInvalid) {
		t.Fatalf("bad mac: %v", err)
	}
	unsigned := signedForm(fields, testSalt)
	unsigned.Del("mac")
	if _, err := g.HandleWebhook(context.Background(), WebhookRequest{Form: unsigned}); !errors.Is(err, ErrWebhookSignatureInvalid) {
		t.Fatalf("missing mac: %v", err)
	}
	tampered := signedForm(fields, testSalt)
	tampered.Set("amount", "1.00")
	if _, err := g.HandleWebhook(context.Background(), WebhookRequest{Form: tampered}); !errors.Is(err, ErrWebhookSignatureInvalid) {
		t.Fatalf("tampered: %v", err)
	}
	fake.pay(resp.Payment.GatewayReference, "MOJO5a06005J21512197", boolPtr(false), "1499.00")
	payment, err := g.HandleWebhook(context.Background(), WebhookRequest{Form: signedForm(fields, testSalt)})
	if err != nil || payment.State != StateAwaitingPayment {
		t.Fatalf("webhook says Credit but instamojo says failed: %v %s", err, payment.State)
	}
	fake.mu.Lock()
	fake.payments["MOJO5a06005J21512197"] = instamojoClient.Payment{ID: "MOJO5a06005J21512197", PaymentRequest: "https://api.instamojo.com/v2/payment_requests/" + resp.Payment.GatewayReference + "/", Status: boolPtr(true), Currency: "INR", Amount: "1499.00"}
	fake.mu.Unlock()
	calls := fake.getPaymentCalls.Load()
	fields["status"] = "Credit"
	fields["fees"] = "30.00"
	payment, err = g.HandleWebhook(context.Background(), WebhookRequest{Form: signedForm(fields, testSalt)})
	if err != nil || payment.State != StateSucceeded || fake.getPaymentCalls.Load() != calls+1 {
		t.Fatalf("verified webhook: %v %s", err, payment.State)
	}
	again, err := g.HandleWebhook(context.Background(), WebhookRequest{Form: signedForm(fields, testSalt)})
	if err != nil || again.State != StateSucceeded {
		t.Fatalf("repeat webhook: %v", err)
	}
	model, _ := repo.GetPayment(context.Background(), resp.Payment.ID)
	if !hasEvent(model, "webhook_received") {
		t.Fatalf("webhook event not stored")
	}
	if got := recorder.snapshot(); len(got) != 1 || got[0] != StateSucceeded {
		t.Fatalf("finalized %v", got)
	}
	wrongPurpose := map[string]string{"payment_id": "MOJO1", "payment_request_id": resp.Payment.GatewayReference, "purpose": "someone-else", "status": "Credit"}
	if _, err := g.HandleWebhook(context.Background(), WebhookRequest{Form: signedForm(wrongPurpose, testSalt)}); !errors.Is(err, ErrWebhookInvalid) {
		t.Fatalf("wrong purpose: %v", err)
	}
	unknown := map[string]string{"payment_id": "MOJO1", "payment_request_id": strings.Repeat("f", 32), "status": "Credit"}
	if _, err := g.HandleWebhook(context.Background(), WebhookRequest{Form: signedForm(unknown, testSalt)}); !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf("unknown request: %v", err)
	}
	missing := map[string]string{"payment_request_id": resp.Payment.GatewayReference, "status": "Credit"}
	if _, err := g.HandleWebhook(context.Background(), WebhookRequest{Form: signedForm(missing, testSalt)}); !errors.Is(err, ErrWebhookInvalid) {
		t.Fatalf("missing payment id: %v", err)
	}
	doubled := signedForm(fields, testSalt)
	doubled.Add("status", "Failed")
	if _, err := g.HandleWebhook(context.Background(), WebhookRequest{Form: doubled}); !errors.Is(err, ErrWebhookInvalid) {
		t.Fatalf("repeated field: %v", err)
	}
}

func TestInstamojo_LateWebhookAfterExpiry(t *testing.T) {
	g, fake, _, recorder := newInstamojo(t, nil)
	resp, err := g.InitiatePayment(context.Background(), initiateRequest("order-1"))
	if err != nil {
		t.Fatalf("Initiate: %v", err)
	}
	eventually(t, "expiry", stateIs(t, g, resp.Payment.ID, StateExpired))
	fake.pay(resp.Payment.GatewayReference, "MOJOLATE000000000001", boolPtr(true), "1499.00")
	fields := map[string]string{"payment_id": "MOJOLATE000000000001", "payment_request_id": resp.Payment.GatewayReference, "status": "Credit"}
	payment, err := g.HandleWebhook(context.Background(), WebhookRequest{Form: signedForm(fields, testSalt)})
	if err != nil || payment.State != StateSucceeded {
		t.Fatalf("late webhook: %v %s", err, payment.State)
	}
	if got := recorder.snapshot(); len(got) != 2 || got[0] != StateExpired || got[1] != StateSucceeded {
		t.Fatalf("finalized %v", got)
	}
}

func TestInstamojo_RedirectNeverClosesButSpeedsUpVerification(t *testing.T) {
	g, fake, repo, _ := newInstamojo(t, func(p *InstamojoConstructorParams) { p.PollInterval, p.PollWindow = time.Hour, 2*time.Hour })
	resp, err := g.InitiatePayment(context.Background(), initiateRequest("order-1"))
	if err != nil {
		t.Fatalf("Initiate: %v", err)
	}
	query := url.Values{"payment_request_id": {resp.Payment.GatewayReference}, "payment_id": {"MOJOREDIRECT00000001"}, "payment_status": {"Credit"}}
	payment, err := g.HandleRedirect(context.Background(), RedirectRequest{Query: query})
	if err != nil || payment.State != StateAwaitingPayment || payment.Metadata["redirect_called"] != true || payment.Metadata["redirect_status"] != "Credit" {
		t.Fatalf("redirect: %v %+v", err, payment)
	}
	time.Sleep(200 * time.Millisecond)
	if current, _ := g.GetPayment(context.Background(), resp.Payment.ID); current.State != StateAwaitingPayment {
		t.Fatalf("redirect alone must never close the payment, got %s", current.State)
	}
	fake.pay(resp.Payment.GatewayReference, "MOJOREDIRECT00000001", boolPtr(true), "1499.00")
	if _, err := g.HandleRedirect(context.Background(), RedirectRequest{Query: query}); err != nil {
		t.Fatalf("repeat redirect: %v", err)
	}
	eventually(t, "verification triggered by redirect", stateIs(t, g, resp.Payment.ID, StateSucceeded))
	model, _ := repo.GetPayment(context.Background(), resp.Payment.ID)
	if !hasEvent(model, "redirect_called") {
		t.Fatalf("redirect event missing")
	}
	if _, err := g.HandleRedirect(context.Background(), RedirectRequest{Query: url.Values{}}); !errors.Is(err, ErrRedirectInvalid) {
		t.Fatalf("empty redirect: %v", err)
	}
	if _, err := g.HandleRedirect(context.Background(), RedirectRequest{Query: url.Values{"payment_request_id": {strings.Repeat("e", 32)}}}); !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf("unknown redirect: %v", err)
	}
}

func TestInstamojo_WebhookAndPollRaceFinalizeOnce(t *testing.T) {
	for round := range 5 {
		g, fake, _, recorder := newInstamojo(t, func(p *InstamojoConstructorParams) {
			p.PollInterval, p.PollWindow = 5*time.Millisecond, 5*time.Second
		})
		resp, err := g.InitiatePayment(context.Background(), initiateRequest(fmt.Sprintf("order-%d", round)))
		if err != nil {
			t.Fatalf("Initiate: %v", err)
		}
		fake.pay(resp.Payment.GatewayReference, "MOJORACE000000000001", boolPtr(true), "1499.00")
		fields := map[string]string{"payment_id": "MOJORACE000000000001", "payment_request_id": resp.Payment.GatewayReference, "status": "Credit"}
		var wg sync.WaitGroup
		for range 4 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := g.HandleWebhook(context.Background(), WebhookRequest{Form: signedForm(fields, testSalt)}); err != nil {
					t.Errorf("webhook: %v", err)
				}
			}()
		}
		wg.Wait()
		eventually(t, "success", stateIs(t, g, resp.Payment.ID, StateSucceeded))
		time.Sleep(50 * time.Millisecond)
		if got := recorder.snapshot(); len(got) != 1 || got[0] != StateSucceeded {
			t.Fatalf("round %d finalized %v", round, got)
		}
	}
}

func TestInstamojo_SubmitProofKeepsVerifying(t *testing.T) {
	g, fake, _, _ := newInstamojo(t, func(p *InstamojoConstructorParams) { p.PollWindow = 5 * time.Second })
	resp, err := g.InitiatePayment(context.Background(), initiateRequest("order-1"))
	if err != nil {
		t.Fatalf("Initiate: %v", err)
	}
	if _, err := g.SubmitProof(context.Background(), SubmitProofRequest{PaymentID: resp.Payment.ID, UserID: "intruder", TransactionID: "412345678901", Image: jpegProof}); !errors.Is(err, ErrPaymentForbidden) {
		t.Fatalf("other user: %v", err)
	}
	payment, err := g.SubmitProof(context.Background(), SubmitProofRequest{PaymentID: resp.Payment.ID, UserID: "user-1", TransactionID: "412345678901", Image: jpegProof})
	if err != nil || payment.State != StateVerifying || payment.ProofTransactionID != "412345678901" || payment.ProofSubmittedAt == nil {
		t.Fatalf("proof: %v %+v", err, payment)
	}
	fake.pay(resp.Payment.GatewayReference, "MOJOPROOF00000000001", boolPtr(true), "1499.00")
	eventually(t, "success after proof", stateIs(t, g, resp.Payment.ID, StateSucceeded))
}

func TestInstamojo_CloseStopsEverything(t *testing.T) {
	g, fake, _, _ := newInstamojo(t, nil)
	if err := g.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := g.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
	calls := []func() error{
		func() error { _, err := g.InitiatePayment(context.Background(), initiateRequest("o")); return err },
		func() error { _, err := g.GetPayment(context.Background(), utils.NewULID()); return err },
		func() error { _, err := g.SubmitProof(context.Background(), SubmitProofRequest{}); return err },
		func() error { _, err := g.HandleWebhook(context.Background(), WebhookRequest{}); return err },
		func() error { _, err := g.HandleRedirect(context.Background(), RedirectRequest{}); return err },
	}
	for i, call := range calls {
		if err := call(); !errors.Is(err, ErrClosed) {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if fake.createCalls.Load() != 0 {
		t.Fatalf("closed gateway must not call instamojo")
	}
}

func TestInstamojo_PollIgnoresJunk(t *testing.T) {
	g, _, _, _ := newInstamojo(t, nil)
	for _, body := range []string{"not json", `{"payment_id":"x"}`, `{"payment_id":"` + utils.NewULID() + `","deadline":1}`} {
		if err := g.poll(context.Background(), []byte(body)); err != nil {
			t.Fatalf("body %q: %v", body, err)
		}
	}
}

func newUPI(t *testing.T) (*selfUPIGateway, paymentsRepository.Repository, *finalizedRecorder) {
	t.Helper()
	repo, _ := newPayments(t)
	recorder := &finalizedRecorder{}
	g, err := NewGateway(UPIConstructorParams{Client: fakeUPIClient{}, Payments: repo, PayeeName: "  Aesthetic   Indians ", OnFinalized: recorder.hook})
	if err != nil {
		t.Fatalf("NewGateway: %v", err)
	}
	t.Cleanup(func() { _ = g.Close() })
	return g.(*selfUPIGateway), repo, recorder
}

func TestUPI_Initiate(t *testing.T) {
	g, _, _ := newUPI(t)
	resp, err := g.InitiatePayment(context.Background(), initiateRequest("order-1"))
	if err != nil {
		t.Fatalf("Initiate: %v", err)
	}
	action := resp.Action
	if action.Type != ActionShowQR || action.QRImageDataURI != "data:image/jpeg;base64,QUJD" || action.UPIID != "shop@ybl" || resp.Payment.State != StateAwaitingPayment || resp.Payment.Provider != ProviderSelfUPI {
		t.Fatalf("response %+v", resp)
	}
	want := "upi://pay?pa=shop@ybl&pn=Aesthetic%20Indians&am=1499.00&cu=INR&tr=" + resp.Payment.ID + "&tn=Order%20order-1"
	if action.UPIIntentURL != want {
		t.Fatalf("intent\n got %s\nwant %s", action.UPIIntentURL, want)
	}
	again, err := g.InitiatePayment(context.Background(), initiateRequest("order-1"))
	if err != nil || again.Payment.ID != resp.Payment.ID || again.Action.UPIIntentURL != want {
		t.Fatalf("idempotent: %v", err)
	}
	high := initiateRequest("order-2")
	high.Amount = 100001
	if _, err := g.InitiatePayment(context.Background(), high); !errors.Is(err, ErrAmountInvalid) {
		t.Fatalf("over limit: %v", err)
	}
	low := initiateRequest("order-3")
	low.Amount = 1
	if _, err := g.InitiatePayment(context.Background(), low); err != nil {
		t.Fatalf("one rupee: %v", err)
	}
}

func TestUPI_ProofAndAdminDecision(t *testing.T) {
	g, repo, recorder := newUPI(t)
	resp, err := g.InitiatePayment(context.Background(), initiateRequest("order-1"))
	if err != nil {
		t.Fatalf("Initiate: %v", err)
	}
	if _, err := g.SubmitProof(context.Background(), SubmitProofRequest{PaymentID: "bad", UserID: "user-1"}); !errors.Is(err, ErrPaymentIDInvalid) {
		t.Fatalf("bad id: %v", err)
	}
	if _, err := g.SubmitProof(context.Background(), SubmitProofRequest{PaymentID: resp.Payment.ID, UserID: "user-1", TransactionID: "12", Image: jpegProof}); !errors.Is(err, paymentsRepository.ErrProofTransactionID) {
		t.Fatalf("bad txn: %v", err)
	}
	payment, err := g.SubmitProof(context.Background(), SubmitProofRequest{PaymentID: resp.Payment.ID, UserID: "user-1", TransactionID: "412345678901", Image: jpegProof})
	if err != nil || payment.State != StateVerifying {
		t.Fatalf("proof: %v", err)
	}
	image, mime, err := repo.GetProofImage(context.Background(), resp.Payment.ID)
	if err != nil || mime != "image/jpeg" || !bytes.Equal(image, jpegProof) {
		t.Fatalf("stored image: %v", err)
	}
	if _, err := g.HandleWebhook(context.Background(), WebhookRequest{Form: url.Values{"payment_id": {resp.Payment.ID}, "status": {"MAYBE"}}}); !errors.Is(err, ErrStatusInvalid) {
		t.Fatalf("bad status: %v", err)
	}
	if _, err := g.HandleWebhook(context.Background(), WebhookRequest{Form: url.Values{"payment_id": {resp.Payment.ID}, "status": {"FAILED"}, "note": {strings.Repeat("n", 301)}}}); !errors.Is(err, ErrNoteInvalid) {
		t.Fatalf("long note: %v", err)
	}
	payment, err = g.HandleWebhook(context.Background(), WebhookRequest{Form: url.Values{"payment_id": {resp.Payment.ID}, "status": {"failed"}, "note": {"amount not received"}}})
	if err != nil || payment.State != StateFailed || payment.FailureReason != "amount not received" {
		t.Fatalf("admin failed: %v %+v", err, payment)
	}
	payment, err = g.HandleWebhook(context.Background(), WebhookRequest{Form: url.Values{"payment_id": {resp.Payment.ID}, "status": {"SUCCEEDED"}}})
	if err != nil || payment.State != StateSucceeded || payment.FailureReason != "" {
		t.Fatalf("admin corrected to success: %v %+v", err, payment)
	}
	if _, err := g.HandleWebhook(context.Background(), WebhookRequest{Form: url.Values{"payment_id": {resp.Payment.ID}, "status": {"FAILED"}}}); !errors.Is(err, paymentsRepository.ErrTransitionNotAllowed) {
		t.Fatalf("success is final: %v", err)
	}
	if got := recorder.snapshot(); len(got) != 2 || got[0] != StateFailed || got[1] != StateSucceeded {
		t.Fatalf("finalized %v", got)
	}
	if _, err := g.HandleRedirect(context.Background(), RedirectRequest{}); !errors.Is(err, ErrNotSupported) {
		t.Fatalf("redirect: %v", err)
	}
}

func TestProviderIsolation(t *testing.T) {
	upi, repo, _ := newUPI(t)
	resp, err := upi.InitiatePayment(context.Background(), initiateRequest("order-1"))
	if err != nil {
		t.Fatalf("Initiate: %v", err)
	}
	g, err := NewGateway(InstamojoConstructorParams{Client: newFakeInstamojo(), Payments: repo, PrivateSalt: testSalt, QueuePath: []string{t.TempDir(), "q.db"}, QueuePermission: 0o600, PollInterval: time.Hour, PollWindow: time.Hour, LeaseTimeout: time.Second, Concurrency: 1})
	if err != nil {
		t.Fatalf("instamojo: %v", err)
	}
	defer g.Close()
	if _, err := g.GetPayment(context.Background(), resp.Payment.ID); !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf("instamojo gateway must not see upi payments: %v", err)
	}
	if _, err := upi.GetPayment(context.Background(), resp.Payment.ID); err != nil {
		t.Fatalf("upi get: %v", err)
	}
}

func TestContextChecks(t *testing.T) {
	g, _, _, _ := newInstamojo(t, nil)
	u, _, _ := newUPI(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, gw := range []Gateway{g, u} {
		calls := []func() error{
			func() error { _, err := gw.InitiatePayment(ctx, initiateRequest("o")); return err },
			func() error { _, err := gw.GetPayment(ctx, utils.NewULID()); return err },
			func() error { _, err := gw.SubmitProof(ctx, SubmitProofRequest{}); return err },
			func() error { _, err := gw.HandleWebhook(ctx, WebhookRequest{}); return err },
			func() error { _, err := gw.HandleRedirect(ctx, RedirectRequest{}); return err },
		}
		for i, call := range calls {
			if err := call(); !errors.Is(err, ErrCtxInvalid) {
				t.Fatalf("%s call %d: %v", gw.Provider(), i, err)
			}
		}
	}
}

func TestFinalizedHookPanicIsContained(t *testing.T) {
	repo, _ := newPayments(t)
	g, err := NewGateway(UPIConstructorParams{Client: fakeUPIClient{}, Payments: repo, PayeeName: "Shop", OnFinalized: func(context.Context, Payment) { panic("boom") }})
	if err != nil {
		t.Fatalf("NewGateway: %v", err)
	}
	resp, err := g.InitiatePayment(context.Background(), initiateRequest("order-1"))
	if err != nil {
		t.Fatalf("Initiate: %v", err)
	}
	payment, err := g.HandleWebhook(context.Background(), WebhookRequest{Form: url.Values{"payment_id": {resp.Payment.ID}, "status": {"SUCCEEDED"}}})
	if err != nil || payment.State != StateSucceeded {
		t.Fatalf("hook panic must not fail the transition: %v", err)
	}
}

func TestHelpers(t *testing.T) {
	cases := map[string]bool{"1499.00": true, "1499": true, "1499.0": true, "1499.50": false, "1500.00": false, "abc": false, "": false}
	for in, want := range cases {
		if amountMatches(in, 1499) != want {
			t.Fatalf("amountMatches(%q)", in)
		}
	}
	if lastPathSegment("https://api.instamojo.com/v2/payments/MOJO1/") != "MOJO1" || lastPathSegment("%zz") != "" {
		t.Fatalf("lastPathSegment")
	}
	if truncate(" a\x00b ", 10) != "a b" || truncate(strings.Repeat("é", 5), 3) != "ééé" {
		t.Fatalf("truncate")
	}
	if !ActionRedirect.IsValid() || ActionType("x").IsValid() || ActionShowQR.String() != "SHOW_QR" {
		t.Fatalf("action type")
	}
	if clampDuration(time.Hour, time.Millisecond, time.Second) != time.Second || clampDuration(0, time.Millisecond, time.Second) != time.Millisecond {
		t.Fatalf("clamp")
	}
	mac := instamojoMAC(map[string]string{"b": "2", "A": "1", "mac": "ignored"}, "salt")
	if mac != instamojoMAC(map[string]string{"A": "1", "b": "2"}, "salt") || len(mac) != 40 {
		t.Fatalf("mac ordering")
	}
}
