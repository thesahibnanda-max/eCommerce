package smtp

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	goMail "github.com/wneessen/go-mail"
)

type Security int

const (
	SecurityStartTLS Security = iota
	SecuritySSL
	SecurityNone
)

type SenderConfig struct {
	Email    string
	Host     string
	Port     int
	Username string
	Password string
	Security Security
	Timeout  time.Duration
}

type sender struct {
	email  string
	client *goMail.Client
}

type SendMailRequest struct {
	To       string `json:"to"`
	Subject  string `json:"subject"`
	HTMLBody string `json:"html_body"`
	Body     string `json:"body"`
}

type Mailer interface {
	SendMail(ctx context.Context, request SendMailRequest) error
}

type mailer struct {
	senders []sender
	next    atomic.Uint64
}

func NewMailer(configs ...SenderConfig) (Mailer, error) {
	if len(configs) == 0 {
		return nil, ErrNoSenders
	}
	senders := make([]sender, len(configs))
	for i, cfg := range configs {
		s, err := newSender(cfg)
		if err != nil {
			return nil, fmt.Errorf("%w: sender %d: %w", ErrSenderInvalid, i, err)
		}
		senders[i] = s
	}
	return &mailer{senders: senders}, nil
}

func (m *mailer) SendMail(ctx context.Context, request SendMailRequest) error {
	if err := utils.CheckContext(ctx); err != nil {
		return err
	}
	message, err := buildMessage(request)
	if err != nil {
		return err
	}

	s := m.senders[(m.next.Add(1)-1)%uint64(len(m.senders))]
	if err := message.From(s.email); err != nil {
		return fmt.Errorf("%w: %w", ErrSenderInvalid, err)
	}
	if err := s.client.DialAndSendWithContext(ctx, message); err != nil {
		return fmt.Errorf("%w: %w", ErrSendFailed, err)
	}
	return nil
}
