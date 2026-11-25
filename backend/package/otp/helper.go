package otp

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/thesahibnanda-max/eCommerce/backend/package/smtp"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

func withDefaults(cfg Config) (Config, error) {
	if cfg.Length == 0 {
		cfg.Length = defaultLength
	}
	if cfg.Length < minLength || cfg.Length > maxLength {
		return Config{}, fmt.Errorf("%w: length must be between %d and %d", ErrConfigInvalid, minLength, maxLength)
	}
	if cfg.TTL == 0 {
		cfg.TTL = defaultTTL
	}
	if cfg.ResendCooldown == 0 {
		cfg.ResendCooldown = defaultResendCooldown
	}
	if cfg.MaxAttempts == 0 {
		cfg.MaxAttempts = defaultMaxAttempts
	}
	if cfg.Capacity == 0 {
		cfg.Capacity = defaultCapacity
	}
	cfg.Subject = strings.TrimSpace(cfg.Subject)
	if cfg.Subject == "" {
		cfg.Subject = defaultSubject
	}
	if cfg.TTL < 0 || cfg.ResendCooldown < 0 || cfg.MaxAttempts < 0 {
		return Config{}, fmt.Errorf("%w: ttl, resend cooldown and max attempts must not be negative", ErrConfigInvalid)
	}
	if cfg.ResendCooldown > cfg.TTL {
		return Config{}, fmt.Errorf("%w: resend cooldown must not exceed ttl", ErrConfigInvalid)
	}
	return cfg, nil
}

func normalizeRecipient(recipient string) (string, error) {
	email, err := utils.ValidateAndNormalizeEmail(recipient)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrRecipientInvalid, err)
	}
	return email.UserName + "@" + email.Domain, nil
}

func (m *mailOTPImpl) reserve(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, busy := m.inFlight[key]; busy {
		return ErrResendTooSoon
	}
	if item := m.cache.Get(key); item != nil {
		now, err := m.now()
		if err != nil {
			return err
		}
		if now.Before(item.Value().sentAt.Add(m.cfg.ResendCooldown)) {
			return ErrResendTooSoon
		}
	}
	m.inFlight[key] = struct{}{}
	return nil
}

func (m *mailOTPImpl) release(key string) {
	m.mu.Lock()
	delete(m.inFlight, key)
	m.mu.Unlock()
}

func (m *mailOTPImpl) cacheTTL() time.Duration {
	return 2 * m.cfg.TTL
}

func buildMail(to string, otp string, cfg Config) smtp.SendMailRequest {
	expiresIn := cfg.TTL.String()
	return smtp.SendMailRequest{
		To:       to,
		Subject:  cfg.Subject,
		HTMLBody: "<p>Your OTP is: <strong>" + html.EscapeString(otp) + "</strong></p><p>This OTP will expire in " + html.EscapeString(expiresIn) + ".</p>",
		Body:     "Your OTP is: " + otp + "\nThis OTP will expire in " + expiresIn + ".",
	}
}
