package smtp_test

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/thesahibnanda-max/eCommerce/backend/package/smtp"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

type received struct {
	from string
	to   []string
	data string
}

type backend struct {
	mu       sync.Mutex
	messages []received
}

func (b *backend) NewSession(*gosmtp.Conn) (gosmtp.Session, error) {
	return &session{backend: b}, nil
}

func (b *backend) all() []received {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]received(nil), b.messages...)
}

type session struct {
	backend *backend
	current received
}

func (s *session) Mail(from string, _ *gosmtp.MailOptions) error {
	s.current.from = from
	return nil
}

func (s *session) Rcpt(to string, _ *gosmtp.RcptOptions) error {
	s.current.to = append(s.current.to, to)
	return nil
}

func (s *session) Data(r io.Reader) error {
	raw, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	s.current.data = string(raw)
	s.backend.mu.Lock()
	s.backend.messages = append(s.backend.messages, s.current)
	s.backend.mu.Unlock()
	s.current = received{}
	return nil
}

func (s *session) Reset()        { s.current = received{} }
func (s *session) Logout() error { return nil }

func startServer(t *testing.T) (*backend, int) {
	t.Helper()
	be := &backend{}
	srv := gosmtp.NewServer(be)
	srv.Domain = "localhost"
	srv.ReadTimeout = 5 * time.Second
	srv.WriteTimeout = 5 * time.Second
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = srv.Close() })
	return be, l.Addr().(*net.TCPAddr).Port
}

func plainConfig(email string, port int) smtp.SenderConfig {
	return smtp.SenderConfig{Email: email, Host: "127.0.0.1", Port: port, Security: smtp.SecurityNone, Timeout: 5 * time.Second}
}

func validRequest() smtp.SendMailRequest {
	return smtp.SendMailRequest{To: "buyer@example.com", Subject: "Order confirmed", Body: "Thanks!", HTMLBody: "<p>Thanks!</p>"}
}

func TestNewMailer(t *testing.T) {
	_, port := startServer(t)
	good := plainConfig("shop@example.com", port)
	with := func(edit func(*smtp.SenderConfig)) smtp.SenderConfig {
		c := good
		edit(&c)
		return c
	}

	tests := []struct {
		name    string
		configs []smtp.SenderConfig
		wantErr error
	}{
		{"no senders", nil, smtp.ErrNoSenders},
		{"empty email", []smtp.SenderConfig{with(func(c *smtp.SenderConfig) { c.Email = "" })}, smtp.ErrSenderInvalid},
		{"bad email", []smtp.SenderConfig{with(func(c *smtp.SenderConfig) { c.Email = "nope" })}, smtp.ErrSenderInvalid},
		{"missing host", []smtp.SenderConfig{with(func(c *smtp.SenderConfig) { c.Host = "  " })}, smtp.ErrSenderInvalid},
		{"negative port", []smtp.SenderConfig{with(func(c *smtp.SenderConfig) { c.Port = -1 })}, smtp.ErrSenderInvalid},
		{"port too large", []smtp.SenderConfig{with(func(c *smtp.SenderConfig) { c.Port = 70000 })}, smtp.ErrSenderInvalid},
		{"password without username", []smtp.SenderConfig{with(func(c *smtp.SenderConfig) { c.Password = "secret" })}, smtp.ErrSenderInvalid},
		{"username without password", []smtp.SenderConfig{with(func(c *smtp.SenderConfig) { c.Username = "user" })}, smtp.ErrSenderInvalid},
		{"unknown security", []smtp.SenderConfig{with(func(c *smtp.SenderConfig) { c.Security = smtp.Security(99) })}, smtp.ErrSenderInvalid},
		{"one bad among good", []smtp.SenderConfig{good, with(func(c *smtp.SenderConfig) { c.Host = "" })}, smtp.ErrSenderInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := smtp.NewMailer(tt.configs...)
			if !errors.Is(err, tt.wantErr) || m != nil {
				t.Errorf("NewMailer = %v, %v; want nil, %v", m, err, tt.wantErr)
			}
		})
	}

	valid := []smtp.SenderConfig{
		good,
		with(func(c *smtp.SenderConfig) { c.Email = "Shop <shop@example.com>" }),
		with(func(c *smtp.SenderConfig) { c.Port, c.Security = 0, smtp.SecurityStartTLS }),
		with(func(c *smtp.SenderConfig) { c.Port, c.Security = 0, smtp.SecuritySSL }),
		with(func(c *smtp.SenderConfig) {
			c.Username, c.Password, c.Security = "user", "secret", smtp.SecurityStartTLS
		}),
		with(func(c *smtp.SenderConfig) { c.Timeout = 0 }),
	}
	for i, cfg := range valid {
		if _, err := smtp.NewMailer(cfg); err != nil {
			t.Errorf("valid config %d rejected: %v", i, err)
		}
	}
}

func TestSendMail_StartTLSIsMandatory(t *testing.T) {
	be, port := startServer(t)
	cfg := plainConfig("shop@example.com", port)
	cfg.Security = smtp.SecurityStartTLS
	m, err := smtp.NewMailer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SendMail(context.Background(), validRequest()); !errors.Is(err, smtp.ErrSendFailed) {
		t.Errorf("err = %v, want ErrSendFailed when the server has no STARTTLS", err)
	}
	if n := len(be.all()); n != 0 {
		t.Errorf("message sent over an unencrypted connection (%d delivered)", n)
	}
}

func TestSendMail_DeliversMessage(t *testing.T) {
	be, port := startServer(t)
	m, err := smtp.NewMailer(plainConfig("shop@example.com", port))
	if err != nil {
		t.Fatal(err)
	}

	if err := m.SendMail(context.Background(), validRequest()); err != nil {
		t.Fatalf("SendMail: %v", err)
	}

	msgs := be.all()
	if len(msgs) != 1 {
		t.Fatalf("server received %d messages, want 1", len(msgs))
	}
	got := msgs[0]
	if got.from != "shop@example.com" || len(got.to) != 1 || got.to[0] != "buyer@example.com" {
		t.Errorf("envelope from=%q to=%v", got.from, got.to)
	}
	for _, want := range []string{"Subject: Order confirmed", "multipart/alternative", "text/plain", "text/html", "Thanks!", "<p>Thanks!</p>"} {
		if !strings.Contains(got.data, want) {
			t.Errorf("message missing %q", want)
		}
	}
}

func TestSendMail_BodyVariants(t *testing.T) {
	be, port := startServer(t)
	m, err := smtp.NewMailer(plainConfig("shop@example.com", port))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		body, html  string
		contains    []string
		notContains []string
	}{
		{"plain only", "Hello plain", "", []string{"text/plain", "Hello plain"}, []string{"text/html", "multipart/alternative"}},
		{"html only", "", "<b>Hello html</b>", []string{"text/html", "<b>Hello html</b>"}, []string{"text/plain", "multipart/alternative"}},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := smtp.SendMailRequest{To: "buyer@example.com", Subject: "Hi", Body: tt.body, HTMLBody: tt.html}
			if err := m.SendMail(context.Background(), req); err != nil {
				t.Fatal(err)
			}
			data := be.all()[i].data
			for _, want := range tt.contains {
				if !strings.Contains(data, want) {
					t.Errorf("message missing %q", want)
				}
			}
			for _, unwanted := range tt.notContains {
				if strings.Contains(data, unwanted) {
					t.Errorf("message unexpectedly contains %q", unwanted)
				}
			}
		})
	}
}

func TestSendMail_RoundRobinAcrossSenders(t *testing.T) {
	be1, port1 := startServer(t)
	be2, port2 := startServer(t)
	m, err := smtp.NewMailer(
		plainConfig("one@example.com", port1),
		plainConfig("two@example.com", port2),
	)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 4; i++ {
		if err := m.SendMail(context.Background(), validRequest()); err != nil {
			t.Fatal(err)
		}
	}
	for name, tc := range map[string]struct {
		be   *backend
		from string
	}{"first": {be1, "one@example.com"}, "second": {be2, "two@example.com"}} {
		msgs := tc.be.all()
		if len(msgs) != 2 {
			t.Errorf("%s sender handled %d messages, want 2", name, len(msgs))
		}
		for _, msg := range msgs {
			if msg.from != tc.from {
				t.Errorf("%s server got MAIL FROM %q, want %q", name, msg.from, tc.from)
			}
		}
	}
}

func TestSendMail_ConcurrentSendsShareClientsSafely(t *testing.T) {
	be1, port1 := startServer(t)
	be2, port2 := startServer(t)
	m, err := smtp.NewMailer(
		plainConfig("one@example.com", port1),
		plainConfig("two@example.com", port2),
	)
	if err != nil {
		t.Fatal(err)
	}

	const total = 40
	var wg sync.WaitGroup
	errs := make(chan error, total)
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- m.SendMail(context.Background(), validRequest())
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent SendMail: %v", err)
		}
	}

	n1, n2 := len(be1.all()), len(be2.all())
	if n1+n2 != total || n1 != total/2 || n2 != total/2 {
		t.Errorf("delivered %d + %d, want exactly %d each", n1, n2, total/2)
	}
}

func TestSendMail_ValidationErrors(t *testing.T) {
	be, port := startServer(t)
	m, err := smtp.NewMailer(plainConfig("shop@example.com", port))
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	with := func(edit func(*smtp.SendMailRequest)) smtp.SendMailRequest {
		r := validRequest()
		edit(&r)
		return r
	}

	tests := []struct {
		name    string
		ctx     context.Context
		req     smtp.SendMailRequest
		wantErr error
	}{
		{"nil ctx", nil, validRequest(), utils.ErrContextInvalid},
		{"canceled ctx", canceled, validRequest(), context.Canceled},
		{"missing recipient", context.Background(), with(func(r *smtp.SendMailRequest) { r.To = "  " }), smtp.ErrRecipientRequired},
		{"bad recipient", context.Background(), with(func(r *smtp.SendMailRequest) { r.To = "not-an-email" }), smtp.ErrRecipientInvalid},
		{"header injection in recipient", context.Background(), with(func(r *smtp.SendMailRequest) { r.To = "a@example.com\r\nBcc: victim@example.com" }), smtp.ErrRecipientInvalid},
		{"missing subject", context.Background(), with(func(r *smtp.SendMailRequest) { r.Subject = "" }), smtp.ErrSubjectRequired},
		{"header injection in subject", context.Background(), with(func(r *smtp.SendMailRequest) { r.Subject = "Hi\r\nBcc: victim@example.com" }), smtp.ErrSubjectInvalid},
		{"missing body", context.Background(), with(func(r *smtp.SendMailRequest) { r.Body, r.HTMLBody = " ", "" }), smtp.ErrBodyRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.SendMail(tt.ctx, tt.req)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
	if n := len(be.all()); n != 0 {
		t.Errorf("server received %d messages from invalid requests, want 0", n)
	}
}

func TestSendMail_ServerUnavailable(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()

	m, err := smtp.NewMailer(plainConfig("shop@example.com", port))
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SendMail(context.Background(), validRequest()); !errors.Is(err, smtp.ErrSendFailed) {
		t.Errorf("err = %v, want ErrSendFailed", err)
	}
}
