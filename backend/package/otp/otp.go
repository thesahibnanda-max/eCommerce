package otp

import (
	"context"
	"crypto/subtle"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/thesahibnanda-max/eCommerce/backend/package/smtp"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	defaultLength         = 6
	minLength             = 4
	maxLength             = 12
	defaultTTL            = 10 * time.Minute
	defaultResendCooldown = time.Minute
	defaultMaxAttempts    = 5
	defaultCapacity       = 100_000
	defaultSubject        = "OTP Verification"
)

type OTP interface {
	GenerateAndSendOTP(ctx context.Context, recipient string) error
	VerifyOTP(ctx context.Context, recipient string, otp string) (bool, error)
}

type Config struct {
	Length         uint
	TTL            time.Duration
	ResendCooldown time.Duration
	MaxAttempts    int
	Capacity       uint64
	Subject        string
}

type ttlEntry struct {
	otp      string
	sentAt   time.Time
	expiry   time.Time
	attempts int
}

type mailOTPImpl struct {
	smtp     smtp.Mailer
	mu       sync.Mutex
	cache    *ttlcache.Cache[string, ttlEntry]
	inFlight map[string]struct{}
	cfg      Config
	now      func() (time.Time, error)
}

func NewMailOTP(mailer smtp.Mailer, cfg Config) (OTP, error) {
	if mailer == nil {
		return nil, ErrMailerRequired
	}
	cfg, err := withDefaults(cfg)
	if err != nil {
		return nil, err
	}
	return &mailOTPImpl{
		smtp: mailer,
		cache: ttlcache.New(
			ttlcache.WithCapacity[string, ttlEntry](cfg.Capacity),
			ttlcache.WithDisableTouchOnHit[string, ttlEntry](),
		),
		inFlight: make(map[string]struct{}),
		cfg:      cfg,
		now:      utils.TimeNow,
	}, nil
}

func (m *mailOTPImpl) GenerateAndSendOTP(ctx context.Context, recipient string) error {
	if err := utils.CheckContext(ctx); err != nil {
		return err
	}
	key, err := normalizeRecipient(recipient)
	if err != nil {
		return err
	}

	if err := m.reserve(key); err != nil {
		return err
	}
	defer m.release(key)

	otp, err := utils.GenerateRandomNumberWithLength(m.cfg.Length, true)
	if err != nil {
		return err
	}

	if err := m.smtp.SendMail(ctx, buildMail(key, otp, m.cfg)); err != nil {
		return fmt.Errorf("%w: %w", ErrSendFailed, err)
	}

	now, err := m.now()
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.cache.Set(key, ttlEntry{otp: otp, sentAt: now, expiry: now.Add(m.cfg.TTL)}, m.cacheTTL())
	m.mu.Unlock()
	return nil
}

func (m *mailOTPImpl) VerifyOTP(ctx context.Context, recipient string, otp string) (bool, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return false, err
	}
	key, err := normalizeRecipient(recipient)
	if err != nil {
		return false, err
	}
	otp = strings.TrimSpace(otp)

	m.mu.Lock()
	defer m.mu.Unlock()

	item := m.cache.Get(key)
	if item == nil {
		return false, ErrOTPNotFound
	}
	entry := item.Value()

	now, err := m.now()
	if err != nil {
		return false, err
	}
	if !now.Before(entry.expiry) {
		m.cache.Delete(key)
		return false, ErrOTPExpired
	}

	if subtle.ConstantTimeCompare([]byte(entry.otp), []byte(otp)) != 1 {
		entry.attempts++
		if entry.attempts >= m.cfg.MaxAttempts {
			m.cache.Delete(key)
			return false, ErrTooManyAttempts
		}
		m.cache.Set(key, entry, ttlcache.PreviousOrDefaultTTL)
		return false, ErrOTPInvalid
	}

	m.cache.Delete(key)
	return true, nil
}

type smsOTPImpl struct{}

func (s *smsOTPImpl) GenerateAndSendOTP(ctx context.Context, recipient string) error {
	return ErrSMSNotSupported
}

func (s *smsOTPImpl) VerifyOTP(ctx context.Context, recipient string, otp string) (bool, error) {
	return false, ErrSMSNotSupported
}
