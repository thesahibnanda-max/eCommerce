package instamojoClient

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	minAmount          = 9
	maxAmount          = 200000
	maxPurposeLength   = 30
	maxBuyerNameLength = 100
	tokenRefreshMargin = 60 * time.Second
	maxErrorBodyLength = 512
)

var (
	paymentRequestIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
	paymentIDPattern        = regexp.MustCompile(`^MOJO[0-9A-Za-z]{10,40}$`)
)

func (r CreatePaymentRequestRequest) toForm(now time.Time) (map[string]string, error) {
	if r.Amount < minAmount || r.Amount > maxAmount {
		return nil, ErrAmountInvalid
	}
	purpose, ok := cleanText(r.Purpose, maxPurposeLength)
	if !ok || purpose == "" {
		return nil, ErrPurposeInvalid
	}
	buyerName, ok := cleanText(r.BuyerName, maxBuyerNameLength)
	if !ok {
		return nil, ErrBuyerNameInvalid
	}
	form := map[string]string{
		"amount":                  strconv.FormatInt(r.Amount, 10) + ".00",
		"purpose":                 purpose,
		"allow_repeated_payments": strconv.FormatBool(r.AllowRepeatedPayments),
		"send_email":              strconv.FormatBool(r.SendEmail),
	}
	if buyerName != "" {
		form["buyer_name"] = buyerName
	}
	if email := strings.TrimSpace(r.Email); email != "" {
		normalized, err := utils.ValidateAndNormalizeEmail(email)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrEmailInvalid, err)
		}
		form["email"] = normalized.UserName + "@" + normalized.Domain
	} else if r.SendEmail {
		return nil, ErrEmailRequired
	}
	if phone := strings.TrimSpace(r.Phone); phone != "" {
		parsed, err := utils.ValidatePhoneNumber(phone)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrPhoneInvalid, err)
		}
		form["phone"] = "+" + strconv.FormatUint(uint64(parsed.CountryCode), 10) + strconv.FormatUint(parsed.Number, 10)
	}
	if raw := strings.TrimSpace(r.RedirectURL); raw != "" {
		if !isWebURL(raw) {
			return nil, ErrRedirectURLInvalid
		}
		form["redirect_url"] = raw
	}
	if raw := strings.TrimSpace(r.WebhookURL); raw != "" {
		if !isWebURL(raw) {
			return nil, ErrWebhookURLInvalid
		}
		form["webhook"] = raw
	}
	if !r.ExpiresAt.IsZero() {
		if !r.ExpiresAt.After(now) {
			return nil, ErrExpiresAtInvalid
		}
		form["expires_at"] = r.ExpiresAt.UTC().Format(time.RFC3339)
	}
	return form, nil
}

func cleanText(value string, maxLength int) (string, bool) {
	value = strings.Join(strings.Fields(value), " ")
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > maxLength || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", false
	}
	return value, true
}

func isWebURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return false
	}
	return parsed.Scheme == "https" || parsed.Scheme == "http"
}

func validateBaseURL(raw string) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", ErrConfigInvalid
	}
	switch parsed.Scheme {
	case "https":
		return raw, nil
	case "http":
		if isLoopback(parsed.Hostname()) {
			return raw, nil
		}
	}
	return "", ErrConfigInvalid
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func tokenLifetime(expiresIn int64) time.Duration {
	lifetime := time.Duration(expiresIn) * time.Second
	margin := tokenRefreshMargin
	if margin > lifetime/2 {
		margin = lifetime / 2
	}
	return lifetime - margin
}

func decodeToken(status int, body string) (tokenResponse, error) {
	if status == http.StatusBadRequest || status == http.StatusUnauthorized || status == http.StatusForbidden {
		return tokenResponse{}, fmt.Errorf("%w: status %d: %s", ErrAuthFailed, status, describeErrorBody(body))
	}
	if status != http.StatusOK {
		return tokenResponse{}, fmt.Errorf("%w: token status %d: %s", ErrUpstream, status, describeErrorBody(body))
	}
	token, err := utils.FromStringToObjectJSON[tokenResponse](body)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("%w: %w", ErrResponseInvalid, err)
	}
	token.AccessToken = strings.TrimSpace(token.AccessToken)
	if token.AccessToken == "" || token.ExpiresIn <= 0 {
		return tokenResponse{}, fmt.Errorf("%w: token missing access_token or expires_in", ErrResponseInvalid)
	}
	if token.TokenType != "" && !strings.EqualFold(token.TokenType, "bearer") {
		return tokenResponse{}, fmt.Errorf("%w: unexpected token type %q", ErrResponseInvalid, token.TokenType)
	}
	return token, nil
}

func checkStatus(status int, body string) error {
	switch {
	case status == http.StatusOK || status == http.StatusCreated:
		return nil
	case status == http.StatusUnauthorized:
		return fmt.Errorf("%w: status %d: %s", ErrUnauthorized, status, describeErrorBody(body))
	case status == http.StatusForbidden:
		return fmt.Errorf("%w: status %d: %s", ErrForbidden, status, describeErrorBody(body))
	case status == http.StatusBadRequest:
		return fmt.Errorf("%w: %s", ErrValidationFailed, describeErrorBody(body))
	case status == http.StatusNotFound:
		return fmt.Errorf("%w: %s", ErrNotFound, describeErrorBody(body))
	default:
		return fmt.Errorf("%w: status %d: %s", ErrUpstream, status, describeErrorBody(body))
	}
}

func decodePaymentRequest(status int, body, expectedID string) (PaymentRequest, error) {
	if err := checkStatus(status, body); err != nil {
		return PaymentRequest{}, err
	}
	resp, err := utils.FromStringToObjectJSON[PaymentRequest](body)
	if err != nil {
		return PaymentRequest{}, fmt.Errorf("%w: %w", ErrResponseInvalid, err)
	}
	if resp.ID == "" || resp.LongURL == "" {
		return PaymentRequest{}, fmt.Errorf("%w: missing id or longurl", ErrResponseInvalid)
	}
	if expectedID != "" && resp.ID != expectedID {
		return PaymentRequest{}, fmt.Errorf("%w: asked for payment request %s, got %s", ErrResponseInvalid, expectedID, resp.ID)
	}
	if !resp.Status.IsValid() {
		return PaymentRequest{}, fmt.Errorf("%w: unknown status %q", ErrResponseInvalid, resp.Status)
	}
	if resp.Payments == nil {
		resp.Payments = []string{}
	}
	return resp, nil
}

func decodePayment(status int, body, expectedID string) (Payment, error) {
	if err := checkStatus(status, body); err != nil {
		return Payment{}, err
	}
	resp, err := utils.FromStringToObjectJSON[Payment](body)
	if err != nil {
		return Payment{}, fmt.Errorf("%w: %w", ErrResponseInvalid, err)
	}
	if resp.ID != expectedID {
		return Payment{}, fmt.Errorf("%w: asked for payment %s, got %q", ErrResponseInvalid, expectedID, resp.ID)
	}
	if resp.Amount == "" {
		return Payment{}, fmt.Errorf("%w: missing amount", ErrResponseInvalid)
	}
	if resp.Cases == nil {
		resp.Cases = []any{}
	}
	return resp, nil
}

func describeErrorBody(body string) string {
	parsed, err := utils.FromStringToObjectJSON[map[string]any](body)
	if err != nil || len(parsed) == 0 {
		return truncate(strings.TrimSpace(body))
	}
	if nested, ok := parsed["message"].(map[string]any); ok {
		parsed = nested
	}
	parts := make([]string, 0, len(parsed))
	for key, value := range parsed {
		if key == "success" {
			continue
		}
		parts = append(parts, key+": "+flattenValue(value))
	}
	sort.Strings(parts)
	if len(parts) == 0 {
		return truncate(strings.TrimSpace(body))
	}
	return truncate(strings.Join(parts, "; "))
}

func flattenValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []any:
		items := make([]string, 0, len(v))
		for _, item := range v {
			items = append(items, flattenValue(item))
		}
		return strings.Join(items, ", ")
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		items := make([]string, 0, len(keys))
		for _, key := range keys {
			items = append(items, key+": "+flattenValue(v[key]))
		}
		return strings.Join(items, ", ")
	default:
		return fmt.Sprint(v)
	}
}

func truncate(value string) string {
	if utf8.RuneCountInString(value) <= maxErrorBodyLength {
		return value
	}
	return string([]rune(value)[:maxErrorBodyLength]) + "..."
}

func wrapTransport(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("%w: %w", ErrCtxInvalid, ctxErr)
	}
	return fmt.Errorf("%w: %w", ErrUpstream, err)
}

func recoverAsError(err *error) {
	if r := recover(); r != nil {
		*err = fmt.Errorf("%w: %v", ErrRequestPanicked, r)
	}
}
