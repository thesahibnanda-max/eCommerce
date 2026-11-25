package smtp

import (
	"errors"
	"fmt"
	"strings"
	"time"

	goMail "github.com/wneessen/go-mail"
)

const (
	maxPort             = 65535
	defaultTimeout      = 10 * time.Second
	defaultStartTLSPort = 587
	defaultSSLPort      = 465
	defaultPlainPort    = 25
)

func buildMessage(request SendMailRequest) (*goMail.Msg, error) {
	to := strings.TrimSpace(request.To)
	if to == "" {
		return nil, ErrRecipientRequired
	}
	subject := strings.TrimSpace(request.Subject)
	if subject == "" {
		return nil, ErrSubjectRequired
	}
	if strings.ContainsAny(subject, "\r\n") {
		return nil, ErrSubjectInvalid
	}
	hasText := strings.TrimSpace(request.Body) != ""
	hasHTML := strings.TrimSpace(request.HTMLBody) != ""
	if !hasText && !hasHTML {
		return nil, ErrBodyRequired
	}

	message := goMail.NewMsg()
	if err := message.To(to); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRecipientInvalid, err)
	}
	message.Subject(subject)

	switch {
	case hasText && hasHTML:
		message.SetBodyString(goMail.TypeTextPlain, request.Body)
		message.AddAlternativeString(goMail.TypeTextHTML, request.HTMLBody)
	case hasHTML:
		message.SetBodyString(goMail.TypeTextHTML, request.HTMLBody)
	default:
		message.SetBodyString(goMail.TypeTextPlain, request.Body)
	}
	return message, nil
}

func newSender(cfg SenderConfig) (sender, error) {
	email := strings.TrimSpace(cfg.Email)
	if err := goMail.NewMsg().From(email); err != nil {
		return sender{}, fmt.Errorf("email %q: %w", cfg.Email, err)
	}
	host := strings.TrimSpace(cfg.Host)
	if host == "" {
		return sender{}, errors.New("host is required")
	}
	if cfg.Port < 0 || cfg.Port > maxPort {
		return sender{}, fmt.Errorf("port %d out of range", cfg.Port)
	}
	if cfg.Username == "" && cfg.Password != "" {
		return sender{}, errors.New("password given without username")
	}
	if cfg.Username != "" && cfg.Password == "" {
		return sender{}, errors.New("username given without password")
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	opts := []goMail.Option{goMail.WithTimeout(timeout)}

	port := cfg.Port
	switch cfg.Security {
	case SecurityStartTLS:
		opts = append(opts, goMail.WithTLSPolicy(goMail.TLSMandatory))
		if port == 0 {
			port = defaultStartTLSPort
		}
	case SecuritySSL:
		opts = append(opts, goMail.WithSSL())
		if port == 0 {
			port = defaultSSLPort
		}
	case SecurityNone:
		opts = append(opts, goMail.WithTLSPolicy(goMail.NoTLS))
		if port == 0 {
			port = defaultPlainPort
		}
	default:
		return sender{}, fmt.Errorf("unknown security mode %d", cfg.Security)
	}
	opts = append(opts, goMail.WithPort(port))

	if cfg.Username != "" {
		opts = append(opts,
			goMail.WithSMTPAuth(goMail.SMTPAuthAutoDiscover),
			goMail.WithUsername(cfg.Username),
			goMail.WithPassword(cfg.Password),
		)
	}

	client, err := goMail.NewClient(host, opts...)
	if err != nil {
		return sender{}, err
	}
	return sender{email: email, client: client}, nil
}
