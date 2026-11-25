package otp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/thesahibnanda-max/eCommerce/backend/package/smtp"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

type fakeMailer struct {
	mu      sync.Mutex
	sent    []smtp.SendMailRequest
	err     error
	block   chan struct{}
	started chan struct{}
}

func (f *fakeMailer) SendMail(ctx context.Context, req smtp.SendMailRequest) error {
	if f.started != nil {
		f.started <- struct{}{}
	}
	if f.block != nil {
		<-f.block
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, req)
	return nil
}

func (f *fakeMailer) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent)
}

func (f *fakeMailer) lastCode(t *testing.T) string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sent) == 0 {
		t.Fatal("no mail sent")
	}
	body := f.sent[len(f.sent)-1].Body
	code, _, _ := strings.Cut(strings.TrimPrefix(body, "Your OTP is: "), "\n")
	return code
}

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() (time.Time, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now, nil
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

var bg = context.Background()

func newTestOTP(t *testing.T, mailer *fakeMailer, cfg Config) (*mailOTPImpl, *clock) {
	t.Helper()
	o, err := NewMailOTP(mailer, cfg)
	if err != nil {
		t.Fatalf("NewMailOTP: %v", err)
	}
	impl := o.(*mailOTPImpl)
	c := &clock{now: time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}
	impl.now = c.Now
	return impl, c
}

func TestNewMailOTP(t *testing.T) {
	if _, err := NewMailOTP(nil, Config{}); !errors.Is(err, ErrMailerRequired) {
		t.Errorf("nil mailer err = %v", err)
	}

	invalid := map[string]Config{
		"length too short":   {Length: 3},
		"length too long":    {Length: 13},
		"negative ttl":       {TTL: -time.Second},
		"negative cooldown":  {ResendCooldown: -time.Second},
		"negative attempts":  {MaxAttempts: -1},
		"cooldown above ttl": {TTL: time.Minute, ResendCooldown: 2 * time.Minute},
	}
	for name, cfg := range invalid {
		if o, err := NewMailOTP(&fakeMailer{}, cfg); !errors.Is(err, ErrConfigInvalid) || o != nil {
			t.Errorf("%s: got %v, %v; want ErrConfigInvalid", name, o, err)
		}
	}

	o, err := NewMailOTP(&fakeMailer{}, Config{Subject: "   "})
	if err != nil {
		t.Fatal(err)
	}
	got := o.(*mailOTPImpl).cfg
	want := Config{Length: 6, TTL: 10 * time.Minute, ResendCooldown: time.Minute, MaxAttempts: 5, Capacity: 100_000, Subject: "OTP Verification"}
	if got != want {
		t.Errorf("defaults = %+v, want %+v", got, want)
	}
}

func TestGenerateAndSendOTP_SendsMailAndVerifies(t *testing.T) {
	mailer := &fakeMailer{}
	o, _ := newTestOTP(t, mailer, Config{Length: 8, Subject: "Shop OTP"})

	if err := o.GenerateAndSendOTP(bg, "  Buyer@Example.COM "); err != nil {
		t.Fatalf("GenerateAndSendOTP: %v", err)
	}
	if mailer.count() != 1 {
		t.Fatalf("sent %d mails, want 1", mailer.count())
	}
	mail := mailer.sent[0]
	code := mailer.lastCode(t)
	if mail.To != "buyer@example.com" || mail.Subject != "Shop OTP" {
		t.Errorf("mail To=%q Subject=%q", mail.To, mail.Subject)
	}
	if len(code) != 8 || strings.Trim(code, "0123456789") != "" {
		t.Errorf("code %q is not 8 digits", code)
	}
	if !strings.Contains(mail.HTMLBody, "<strong>"+code+"</strong>") || !strings.Contains(mail.Body, "10m0s") {
		t.Errorf("mail bodies missing code or expiry: %q / %q", mail.Body, mail.HTMLBody)
	}

	ok, err := o.VerifyOTP(bg, "buyer@example.com", " "+code+"\n")
	if err != nil || !ok {
		t.Fatalf("VerifyOTP = %v, %v", ok, err)
	}
	if ok, err := o.VerifyOTP(bg, "buyer@example.com", code); ok || !errors.Is(err, ErrOTPNotFound) {
		t.Errorf("second use = %v, %v; want ErrOTPNotFound (one-time)", ok, err)
	}
}

func TestVerifyOTP_MaxAttempts(t *testing.T) {
	mailer := &fakeMailer{}
	o, _ := newTestOTP(t, mailer, Config{MaxAttempts: 3})
	if err := o.GenerateAndSendOTP(bg, "a@x.com"); err != nil {
		t.Fatal(err)
	}
	code := mailer.lastCode(t)
	wrong := "000000"
	if wrong == code {
		wrong = "111111"
	}

	for i := 1; i <= 2; i++ {
		if ok, err := o.VerifyOTP(bg, "a@x.com", wrong); ok || !errors.Is(err, ErrOTPInvalid) {
			t.Fatalf("attempt %d = %v, %v; want ErrOTPInvalid", i, ok, err)
		}
	}
	if ok, err := o.VerifyOTP(bg, "a@x.com", wrong); ok || !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("3rd wrong attempt = %v, %v; want ErrTooManyAttempts", ok, err)
	}
	if ok, err := o.VerifyOTP(bg, "a@x.com", code); ok || !errors.Is(err, ErrOTPNotFound) {
		t.Errorf("correct code after lockout = %v, %v; want ErrOTPNotFound", ok, err)
	}
}

func TestVerifyOTP_CorrectCodeAfterSomeWrongAttempts(t *testing.T) {
	mailer := &fakeMailer{}
	o, _ := newTestOTP(t, mailer, Config{MaxAttempts: 3})
	if err := o.GenerateAndSendOTP(bg, "a@x.com"); err != nil {
		t.Fatal(err)
	}
	code := mailer.lastCode(t)
	if _, err := o.VerifyOTP(bg, "a@x.com", "not-it"); !errors.Is(err, ErrOTPInvalid) {
		t.Fatal(err)
	}
	if ok, err := o.VerifyOTP(bg, "a@x.com", code); !ok || err != nil {
		t.Errorf("correct code after 1 wrong attempt = %v, %v", ok, err)
	}
}

func TestVerifyOTP_Expiry(t *testing.T) {
	mailer := &fakeMailer{}
	o, c := newTestOTP(t, mailer, Config{TTL: 5 * time.Minute, ResendCooldown: time.Minute})
	if err := o.GenerateAndSendOTP(bg, "a@x.com"); err != nil {
		t.Fatal(err)
	}
	code := mailer.lastCode(t)

	c.Advance(5*time.Minute - time.Second)
	if _, err := o.VerifyOTP(bg, "a@x.com", "wrong"); !errors.Is(err, ErrOTPInvalid) {
		t.Fatalf("before expiry err = %v, want ErrOTPInvalid", err)
	}

	c.Advance(time.Second)
	if ok, err := o.VerifyOTP(bg, "a@x.com", code); ok || !errors.Is(err, ErrOTPExpired) {
		t.Fatalf("at expiry = %v, %v; want ErrOTPExpired", ok, err)
	}
	if _, err := o.VerifyOTP(bg, "a@x.com", code); !errors.Is(err, ErrOTPNotFound) {
		t.Errorf("after expiry cleanup err = %v, want ErrOTPNotFound", err)
	}
}

func TestVerifyOTP_JustBeforeExpiryWorks(t *testing.T) {
	mailer := &fakeMailer{}
	o, c := newTestOTP(t, mailer, Config{TTL: 5 * time.Minute, ResendCooldown: time.Minute})
	if err := o.GenerateAndSendOTP(bg, "a@x.com"); err != nil {
		t.Fatal(err)
	}
	c.Advance(5*time.Minute - time.Nanosecond)
	if ok, err := o.VerifyOTP(bg, "a@x.com", mailer.lastCode(t)); !ok || err != nil {
		t.Errorf("just before expiry = %v, %v", ok, err)
	}
}

func TestGenerateAndSendOTP_ResendCooldown(t *testing.T) {
	mailer := &fakeMailer{}
	o, c := newTestOTP(t, mailer, Config{ResendCooldown: time.Minute})
	if err := o.GenerateAndSendOTP(bg, "a@x.com"); err != nil {
		t.Fatal(err)
	}
	first := mailer.lastCode(t)

	c.Advance(59 * time.Second)
	if err := o.GenerateAndSendOTP(bg, "A@X.com"); !errors.Is(err, ErrResendTooSoon) {
		t.Fatalf("resend within cooldown err = %v, want ErrResendTooSoon", err)
	}
	if mailer.count() != 1 {
		t.Fatalf("sent %d mails, want 1", mailer.count())
	}
	if err := o.GenerateAndSendOTP(bg, "other@x.com"); err != nil {
		t.Errorf("cooldown must be per recipient: %v", err)
	}

	c.Advance(time.Second)
	if err := o.GenerateAndSendOTP(bg, "a@x.com"); err != nil {
		t.Fatalf("resend after cooldown: %v", err)
	}
	second := mailer.lastCode(t)
	if first != second {
		if _, err := o.VerifyOTP(bg, "a@x.com", first); !errors.Is(err, ErrOTPInvalid) {
			t.Errorf("old code after resend err = %v, want ErrOTPInvalid", err)
		}
	}
	if ok, err := o.VerifyOTP(bg, "a@x.com", second); !ok || err != nil {
		t.Errorf("new code = %v, %v", ok, err)
	}
}

func TestGenerateAndSendOTP_SendFailureKeepsPreviousCode(t *testing.T) {
	mailer := &fakeMailer{}
	o, c := newTestOTP(t, mailer, Config{})

	mailer.err = errors.New("smtp down")
	if err := o.GenerateAndSendOTP(bg, "a@x.com"); !errors.Is(err, ErrSendFailed) {
		t.Fatalf("err = %v, want ErrSendFailed", err)
	}
	if _, err := o.VerifyOTP(bg, "a@x.com", "123456"); !errors.Is(err, ErrOTPNotFound) {
		t.Errorf("code stored despite send failure: %v", err)
	}
	if err := o.GenerateAndSendOTP(bg, "a@x.com"); err == nil {
		t.Fatal("expected failure while smtp is down")
	}

	mailer.err = nil
	if err := o.GenerateAndSendOTP(bg, "a@x.com"); err != nil {
		t.Fatalf("failed sends must not start the cooldown: %v", err)
	}
	good := mailer.lastCode(t)

	c.Advance(2 * time.Minute)
	mailer.err = errors.New("smtp down")
	if err := o.GenerateAndSendOTP(bg, "a@x.com"); !errors.Is(err, ErrSendFailed) {
		t.Fatal(err)
	}
	if ok, err := o.VerifyOTP(bg, "a@x.com", good); !ok || err != nil {
		t.Errorf("previous code lost after a failed resend: %v, %v", ok, err)
	}
}

func TestGenerateAndSendOTP_InFlightBlocksDuplicateButNotOthers(t *testing.T) {
	mailer := &fakeMailer{block: make(chan struct{}), started: make(chan struct{}, 4)}
	o, _ := newTestOTP(t, mailer, Config{})

	done := make(chan error, 1)
	go func() { done <- o.GenerateAndSendOTP(bg, "a@x.com") }()
	<-mailer.started

	if err := o.GenerateAndSendOTP(bg, "a@x.com"); !errors.Is(err, ErrResendTooSoon) {
		t.Errorf("duplicate while sending err = %v, want ErrResendTooSoon", err)
	}

	otherDone := make(chan error, 1)
	go func() { otherDone <- o.GenerateAndSendOTP(bg, "b@x.com") }()
	select {
	case <-mailer.started:
	case <-time.After(2 * time.Second):
		t.Fatal("a slow send for one recipient blocked another recipient")
	}

	if _, err := o.VerifyOTP(bg, "c@x.com", "123456"); !errors.Is(err, ErrOTPNotFound) {
		t.Errorf("verify blocked or wrong while a send is in flight: %v", err)
	}

	close(mailer.block)
	if err := <-done; err != nil {
		t.Errorf("first send: %v", err)
	}
	if err := <-otherDone; err != nil {
		t.Errorf("other send: %v", err)
	}
}

func TestOTP_InputErrors(t *testing.T) {
	o, _ := newTestOTP(t, &fakeMailer{}, Config{})
	canceled, cancel := context.WithCancel(bg)
	cancel()

	if err := o.GenerateAndSendOTP(nil, "a@x.com"); !errors.Is(err, utils.ErrContextInvalid) {
		t.Errorf("generate nil ctx err = %v", err)
	}
	if err := o.GenerateAndSendOTP(canceled, "a@x.com"); !errors.Is(err, context.Canceled) {
		t.Errorf("generate canceled ctx err = %v", err)
	}
	if _, err := o.VerifyOTP(nil, "a@x.com", "123456"); !errors.Is(err, utils.ErrContextInvalid) {
		t.Errorf("verify nil ctx err = %v", err)
	}
	for _, bad := range []string{"", "   ", "not-an-email", "a@"} {
		if err := o.GenerateAndSendOTP(bg, bad); !errors.Is(err, ErrRecipientInvalid) {
			t.Errorf("generate %q err = %v, want ErrRecipientInvalid", bad, err)
		}
		if _, err := o.VerifyOTP(bg, bad, "123456"); !errors.Is(err, ErrRecipientInvalid) {
			t.Errorf("verify %q err = %v, want ErrRecipientInvalid", bad, err)
		}
	}
}

func TestOTP_CapacityBoundsMemory(t *testing.T) {
	mailer := &fakeMailer{}
	o, _ := newTestOTP(t, mailer, Config{Capacity: 3})
	for i := 0; i < 10; i++ {
		if err := o.GenerateAndSendOTP(bg, fmt.Sprintf("user%d@x.com", i)); err != nil {
			t.Fatal(err)
		}
	}
	if n := o.cache.Len(); n > 3 {
		t.Errorf("cache holds %d entries, want at most 3", n)
	}
	if ok, err := o.VerifyOTP(bg, "user9@x.com", mailer.lastCode(t)); !ok || err != nil {
		t.Errorf("newest entry evicted: %v, %v", ok, err)
	}
}

func TestOTP_ConcurrentUse(t *testing.T) {
	mailer := &fakeMailer{}
	o, _ := newTestOTP(t, mailer, Config{})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			email := fmt.Sprintf("user%d@x.com", i%10)
			_ = o.GenerateAndSendOTP(bg, email)
			_, _ = o.VerifyOTP(bg, email, "123456")
		}(i)
	}
	wg.Wait()
}

func TestSMSOTPNotSupported(t *testing.T) {
	s := &smsOTPImpl{}
	if err := s.GenerateAndSendOTP(bg, "+919876543210"); !errors.Is(err, ErrSMSNotSupported) {
		t.Errorf("err = %v", err)
	}
	if ok, err := s.VerifyOTP(bg, "+919876543210", "123456"); ok || !errors.Is(err, ErrSMSNotSupported) {
		t.Errorf("verify = %v, %v", ok, err)
	}
}
