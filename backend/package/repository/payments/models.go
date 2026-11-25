package paymentsRepository

import (
	"time"
)

type Model struct {
	ID               string         `json:"id"`
	OrderID          string         `json:"order_id"`
	UserID           string         `json:"user_id"`
	Provider         Provider       `json:"provider"`
	State            State          `json:"state"`
	Amount           int64          `json:"amount"`
	Currency         Currency       `json:"currency"`
	Purpose          string         `json:"purpose"`
	GatewayReference string         `json:"gateway_reference"`
	GatewayPaymentID string         `json:"gateway_payment_id"`
	CheckoutURL      string         `json:"checkout_url"`
	Proof            *Proof         `json:"proof"`
	FailureReason    string         `json:"failure_reason"`
	Metadata         map[string]any `json:"metadata"`
	Events           []Event        `json:"events"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type Proof struct {
	TransactionID string    `json:"transaction_id"`
	ImageMIMEType string    `json:"image_mime_type"`
	SubmittedAt   time.Time `json:"submitted_at"`
}

type Event struct {
	Source    EventSource    `json:"source"`
	Kind      string         `json:"kind"`
	FromState State          `json:"from_state"`
	ToState   State          `json:"to_state"`
	Data      map[string]any `json:"data"`
	CreatedAt time.Time      `json:"created_at"`
}

type CreateInput struct {
	OrderID  string
	UserID   string
	Provider Provider
	Amount   int64
	Purpose  string
	Metadata map[string]any
}

type EventInput struct {
	Source    EventSource
	Kind      string
	Data      map[string]any
	DedupeKey string
}

type Patch struct {
	GatewayReference *string
	GatewayPaymentID *string
	CheckoutURL      *string
	FailureReason    *string
	Metadata         map[string]any
}

type ProofInput struct {
	TransactionID string
	Image         []byte
}

type ListQuery struct {
	States   []State
	Provider Provider
	Limit    int
	Cursor   string
}

type Page struct {
	Payments   []Model `json:"payments"`
	NextCursor string  `json:"next_cursor"`
}

type cursor struct {
	CreatedAt int64  `json:"t"`
	ID        string `json:"i"`
}
