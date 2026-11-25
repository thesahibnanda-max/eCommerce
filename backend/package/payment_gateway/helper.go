package paymentGateway

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	paymentsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/payments"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	maxIDRunes          = 128
	maxPurposeRunes     = 100
	maxNoteRunes        = 300
	maxReasonRunes      = 300
	maxFieldRunes       = 512
	finalizeTimeout     = 10 * time.Second
	bestEffortTimeout   = 5 * time.Second
	metadataDeadlineKey = "poll_deadline"
	webhookMACField     = "mac"
)

type core struct {
	provider    Provider
	payments    paymentsRepository.Repository
	logger      *slog.Logger
	onFinalized FinalizedFunc
	now         func() time.Time
	closed      atomic.Bool
}

func newCore(provider Provider, payments paymentsRepository.Repository, logger *slog.Logger, onFinalized FinalizedFunc) *core {
	return &core{provider: provider, payments: payments, logger: logger, onFinalized: onFinalized, now: time.Now}
}

func (c *core) Provider() Provider {
	return c.provider
}

func (c *core) ready(ctx context.Context) error {
	if err := utils.CheckContext(ctx); err != nil {
		return err
	}
	if c.closed.Load() {
		return ErrClosed
	}
	return nil
}

func (c *core) GetPayment(ctx context.Context, paymentID string) (payment Payment, err error) {
	defer recoverAsError(&err)
	if err := c.ready(ctx); err != nil {
		return Payment{}, err
	}
	model, err := c.load(ctx, paymentID)
	if err != nil {
		return Payment{}, err
	}
	return toPayment(model), nil
}

func (c *core) SubmitProof(ctx context.Context, req SubmitProofRequest) (payment Payment, err error) {
	defer recoverAsError(&err)
	if err := c.ready(ctx); err != nil {
		return Payment{}, err
	}
	model, err := c.load(ctx, req.PaymentID)
	if err != nil {
		return Payment{}, err
	}
	if model.UserID != strings.TrimSpace(req.UserID) {
		return Payment{}, ErrPaymentForbidden
	}
	model, err = c.payments.SubmitProof(ctx, model.ID, paymentsRepository.ProofInput{TransactionID: req.TransactionID, Image: req.Image})
	if err != nil {
		return Payment{}, err
	}
	return toPayment(model), nil
}

func (c *core) load(ctx context.Context, paymentID string) (paymentsRepository.Model, error) {
	paymentID = strings.TrimSpace(paymentID)
	if !utils.ValidateULID(paymentID) {
		return paymentsRepository.Model{}, ErrPaymentIDInvalid
	}
	model, err := c.payments.GetPayment(ctx, paymentID)
	if err != nil {
		return paymentsRepository.Model{}, err
	}
	if model.Provider != c.provider {
		return paymentsRepository.Model{}, ErrPaymentNotFound
	}
	return model, nil
}

func (c *core) create(ctx context.Context, req InitiateRequest, minAmount, maxAmount int64, metadata map[string]any) (paymentsRepository.Model, bool, error) {
	req, err := normalizeInitiate(req, minAmount, maxAmount)
	if err != nil {
		return paymentsRepository.Model{}, false, err
	}
	return c.payments.CreatePayment(ctx, paymentsRepository.CreateInput{
		OrderID:  req.OrderID,
		UserID:   req.UserID,
		Provider: c.provider,
		Amount:   req.Amount,
		Purpose:  req.Purpose,
		Metadata: metadata,
	})
}

func (c *core) transition(ctx context.Context, paymentID string, to State, patch paymentsRepository.Patch, event paymentsRepository.EventInput) (paymentsRepository.Model, bool, error) {
	model, transitioned, err := c.payments.Transition(ctx, paymentID, to, patch, event)
	if err != nil {
		return model, false, err
	}
	if transitioned && to.IsClosed() {
		c.finalize(ctx, model)
	}
	return model, transitioned, nil
}

func (c *core) finalize(ctx context.Context, model paymentsRepository.Model) {
	if c.onFinalized == nil {
		return
	}
	hookCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), finalizeTimeout)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			c.logError("payment finalized hook panicked", "payment_id", model.ID, "panic", fmt.Sprint(recovered))
		}
	}()
	c.onFinalized(hookCtx, toPayment(model))
}

func (c *core) markFailed(ctx context.Context, paymentID, reason string) {
	failCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), bestEffortTimeout)
	defer cancel()
	reason = truncate(reason, maxReasonRunes)
	_, _, err := c.transition(failCtx, paymentID, StateFailed, paymentsRepository.Patch{FailureReason: &reason}, paymentsRepository.EventInput{
		Source: paymentsRepository.EventSourceSystem,
		Kind:   "setup_failed",
		Data:   map[string]any{"reason": reason},
	})
	if err != nil {
		c.logError("could not mark payment failed", "payment_id", paymentID, "error", err.Error())
	}
}

func (c *core) logError(msg string, args ...any) {
	if c.logger != nil {
		c.logger.Error(msg, args...)
	}
}

func normalizeInitiate(req InitiateRequest, minAmount, maxAmount int64) (InitiateRequest, error) {
	orderID, ok := cleanText(req.OrderID, maxIDRunes)
	if !ok || orderID == "" {
		return InitiateRequest{}, ErrOrderIDInvalid
	}
	userID, ok := cleanText(req.UserID, maxIDRunes)
	if !ok || userID == "" {
		return InitiateRequest{}, ErrUserIDInvalid
	}
	if req.Amount < minAmount || req.Amount > maxAmount {
		return InitiateRequest{}, ErrAmountInvalid
	}
	purpose, ok := cleanText(req.Purpose, maxPurposeRunes)
	if !ok {
		return InitiateRequest{}, ErrPurposeInvalid
	}
	req.OrderID, req.UserID, req.Purpose = orderID, userID, purpose
	return req, nil
}

func toPayment(model paymentsRepository.Model) Payment {
	payment := Payment{
		ID:               model.ID,
		OrderID:          model.OrderID,
		UserID:           model.UserID,
		Provider:         model.Provider,
		State:            model.State,
		Amount:           model.Amount,
		Currency:         model.Currency.String(),
		Purpose:          model.Purpose,
		CheckoutURL:      model.CheckoutURL,
		GatewayReference: model.GatewayReference,
		GatewayPaymentID: model.GatewayPaymentID,
		FailureReason:    model.FailureReason,
		Metadata:         model.Metadata,
		CreatedAt:        model.CreatedAt,
		UpdatedAt:        model.UpdatedAt,
	}
	if model.Proof != nil {
		submitted := model.Proof.SubmittedAt
		payment.ProofTransactionID = model.Proof.TransactionID
		payment.ProofSubmittedAt = &submitted
	}
	return payment
}

func cleanText(value string, maxRunes int) (string, bool) {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > maxRunes || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", false
	}
	return value, true
}

func truncate(value string, maxRunes int) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, strings.ToValidUTF8(strings.TrimSpace(value), ""))
	if utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	return string([]rune(value)[:maxRunes])
}

func isWebURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return false
	}
	return parsed.Scheme == "https" || parsed.Scheme == "http"
}

func formatAmount(amount int64) string {
	return strconv.FormatInt(amount, 10) + ".00"
}

func amountMatches(reported string, expected int64) bool {
	reported = strings.TrimSpace(reported)
	whole, fraction, hasFraction := strings.Cut(reported, ".")
	if hasFraction && strings.Trim(fraction, "0") != "" {
		return false
	}
	value, err := strconv.ParseInt(whole, 10, 64)
	return err == nil && value == expected
}

func lastPathSegment(link string) string {
	parsed, err := url.Parse(strings.TrimSpace(link))
	if err != nil {
		return ""
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	return segments[len(segments)-1]
}

func singleValues(values url.Values) (map[string]string, error) {
	fields := make(map[string]string, len(values))
	for key, list := range values {
		if len(list) != 1 {
			return nil, ErrWebhookInvalid
		}
		value, ok := cleanText(list[0], maxFieldRunes)
		if !ok {
			return nil, ErrWebhookInvalid
		}
		fields[key] = value
	}
	return fields, nil
}

func instamojoMAC(fields map[string]string, salt string) string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		if key != webhookMACField {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		left, right := strings.ToLower(keys[i]), strings.ToLower(keys[j])
		if left == right {
			return keys[i] < keys[j]
		}
		return left < right
	})
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, fields[key])
	}
	mac := hmac.New(sha1.New, []byte(salt))
	mac.Write([]byte(strings.Join(values, "|")))
	return hex.EncodeToString(mac.Sum(nil))
}

func verifyInstamojoWebhook(form url.Values, salt string) (map[string]string, error) {
	fields, err := singleValues(form)
	if err != nil {
		return nil, err
	}
	provided := strings.ToLower(fields[webhookMACField])
	if provided == "" {
		return nil, ErrWebhookSignatureInvalid
	}
	expected := instamojoMAC(fields, salt)
	if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		return nil, ErrWebhookSignatureInvalid
	}
	delete(fields, webhookMACField)
	return fields, nil
}

func ignoreDuplicate(err error) error {
	if errors.Is(err, paymentsRepository.ErrEventDuplicate) {
		return nil
	}
	return err
}

func recoverAsError(err *error) {
	if recovered := recover(); recovered != nil {
		*err = fmt.Errorf("%w: %v", ErrPanicked, recovered)
	}
}

func stringMap(fields map[string]string) map[string]any {
	out := make(map[string]any, len(fields))
	for key, value := range fields {
		out[key] = value
	}
	return out
}
