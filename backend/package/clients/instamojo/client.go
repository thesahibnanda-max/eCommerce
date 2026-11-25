package instamojoClient

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	"resty.dev/v3"
)

const (
	tokenPath          = "/oauth2/token/"
	paymentRequestPath = "/v2/payment_requests/"
	paymentPath        = "/v2/payments/"
)

type Client interface {
	CreatePaymentRequest(ctx context.Context, req CreatePaymentRequestRequest) (PaymentRequest, error)
	GetPaymentRequest(ctx context.Context, paymentRequestID string) (PaymentRequest, error)
	GetPaymentDetails(ctx context.Context, paymentID string) (Payment, error)
	Close() error
}

type client struct {
	httpClient   *resty.Client
	clientID     string
	clientSecret string
	token        string
	tokenMu      sync.RWMutex
	tokenExp     time.Time
	closed       atomic.Bool
	now          func() time.Time
}

func NewClient(baseURL, clientID, clientSecret string, timeout time.Duration) (Client, error) {
	baseURL, err := validateBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	clientID = strings.TrimSpace(clientID)
	clientSecret = strings.TrimSpace(clientSecret)
	if clientID == "" || clientSecret == "" || timeout <= 0 {
		return nil, ErrConfigInvalid
	}
	httpClient := resty.New().
		SetBaseURL(baseURL).
		SetTimeout(timeout).
		SetRetryCount(0).
		SetHeader("Accept", "application/json")
	return &client{
		httpClient:   httpClient,
		clientID:     clientID,
		clientSecret: clientSecret,
		now:          time.Now,
	}, nil
}

func (c *client) CreatePaymentRequest(ctx context.Context, req CreatePaymentRequestRequest) (result PaymentRequest, err error) {
	defer recoverAsError(&err)
	if err := c.ready(ctx); err != nil {
		return PaymentRequest{}, err
	}
	form, err := req.toForm(c.now())
	if err != nil {
		return PaymentRequest{}, err
	}
	status, body, err := c.send(ctx, http.MethodPost, paymentRequestPath, form)
	if err != nil {
		return PaymentRequest{}, err
	}
	return decodePaymentRequest(status, body, "")
}

func (c *client) GetPaymentRequest(ctx context.Context, paymentRequestID string) (result PaymentRequest, err error) {
	defer recoverAsError(&err)
	if err := c.ready(ctx); err != nil {
		return PaymentRequest{}, err
	}
	paymentRequestID = strings.TrimSpace(paymentRequestID)
	if !paymentRequestIDPattern.MatchString(paymentRequestID) {
		return PaymentRequest{}, ErrPaymentRequestIDInvalid
	}
	status, body, err := c.send(ctx, http.MethodGet, paymentRequestPath+url.PathEscape(paymentRequestID)+"/", nil)
	if err != nil {
		return PaymentRequest{}, err
	}
	return decodePaymentRequest(status, body, paymentRequestID)
}

func (c *client) GetPaymentDetails(ctx context.Context, paymentID string) (result Payment, err error) {
	defer recoverAsError(&err)
	if err := c.ready(ctx); err != nil {
		return Payment{}, err
	}
	paymentID = strings.TrimSpace(paymentID)
	if !paymentIDPattern.MatchString(paymentID) {
		return Payment{}, ErrPaymentIDInvalid
	}
	status, body, err := c.send(ctx, http.MethodGet, paymentPath+url.PathEscape(paymentID)+"/", nil)
	if err != nil {
		return Payment{}, err
	}
	return decodePayment(status, body, paymentID)
}

func (c *client) Close() error {
	if c.closed.Swap(true) {
		return nil
	}
	c.tokenMu.Lock()
	c.token, c.tokenExp = "", time.Time{}
	c.tokenMu.Unlock()
	return c.httpClient.Close()
}

func (c *client) ready(ctx context.Context) error {
	if err := utils.CheckContext(ctx); err != nil {
		return err
	}
	if c.closed.Load() {
		return ErrClosed
	}
	return nil
}

func (c *client) send(ctx context.Context, method, path string, form map[string]string) (int, string, error) {
	for attempt := 0; ; attempt++ {
		token, err := c.getToken(ctx)
		if err != nil {
			return 0, "", err
		}
		req := c.httpClient.R().SetContext(ctx).SetAuthToken(token)
		if form != nil {
			req.SetFormData(form)
		}
		resp, err := req.Execute(method, path)
		if err != nil {
			return 0, "", wrapTransport(ctx, err)
		}
		if resp.StatusCode() == http.StatusUnauthorized && attempt == 0 {
			c.invalidateToken(token)
			continue
		}
		return resp.StatusCode(), resp.String(), nil
	}
}

func (c *client) getToken(ctx context.Context) (string, error) {
	c.tokenMu.RLock()
	token, exp := c.token, c.tokenExp
	c.tokenMu.RUnlock()
	if token != "" && c.now().Before(exp) {
		return token, nil
	}
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.token != "" && c.now().Before(c.tokenExp) {
		return c.token, nil
	}
	if c.closed.Load() {
		return "", ErrClosed
	}
	resp, err := c.httpClient.R().
		SetContext(ctx).
		SetFormData(map[string]string{
			"grant_type":    "client_credentials",
			"client_id":     c.clientID,
			"client_secret": c.clientSecret,
		}).
		Post(tokenPath)
	if err != nil {
		return "", wrapTransport(ctx, err)
	}
	issued, err := decodeToken(resp.StatusCode(), resp.String())
	if err != nil {
		return "", err
	}
	c.token = issued.AccessToken
	c.tokenExp = c.now().Add(tokenLifetime(issued.ExpiresIn))
	return c.token, nil
}

func (c *client) invalidateToken(stale string) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.token == stale {
		c.token, c.tokenExp = "", time.Time{}
	}
}
