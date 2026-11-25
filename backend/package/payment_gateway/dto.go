package paymentGateway

import (
	"context"
	"net/url"
	"time"
)

type InitiateRequest struct {
	OrderID   string
	UserID    string
	Amount    int64
	Purpose   string
	BuyerName string
	Email     string
	Phone     string
}

type InitiateResponse struct {
	Payment Payment    `json:"payment"`
	Action  NextAction `json:"action"`
}

type NextAction struct {
	Type           ActionType `json:"type"`
	RedirectURL    string     `json:"redirect_url,omitempty"`
	QRImageDataURI string     `json:"qr_image_data_uri,omitempty"`
	UPIID          string     `json:"upi_id,omitempty"`
	UPIIntentURL   string     `json:"upi_intent_url,omitempty"`
}

type SubmitProofRequest struct {
	PaymentID     string
	UserID        string
	TransactionID string
	Image         []byte
}

type WebhookRequest struct {
	Form url.Values
}

type RedirectRequest struct {
	Query url.Values
}

type Payment struct {
	ID                 string         `json:"id"`
	OrderID            string         `json:"order_id"`
	UserID             string         `json:"user_id"`
	Provider           Provider       `json:"provider"`
	State              State          `json:"state"`
	Amount             int64          `json:"amount"`
	Currency           string         `json:"currency"`
	Purpose            string         `json:"purpose"`
	CheckoutURL        string         `json:"checkout_url"`
	GatewayReference   string         `json:"gateway_reference"`
	GatewayPaymentID   string         `json:"gateway_payment_id"`
	ProofTransactionID string         `json:"proof_transaction_id"`
	ProofSubmittedAt   *time.Time     `json:"proof_submitted_at"`
	FailureReason      string         `json:"failure_reason"`
	Metadata           map[string]any `json:"metadata"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type FinalizedFunc func(ctx context.Context, payment Payment)
