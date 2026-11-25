package ordersService

import (
	"net/url"
	"time"
)

type OrderItem struct {
	ProductID        string            `json:"product_id"`
	ProductVariantID string            `json:"product_variant_id"`
	SKU              string            `json:"sku"`
	ProductName      string            `json:"product_name"`
	Attributes       map[string]string `json:"attributes"`
	UnitPrice        int64             `json:"unit_price"`
	Quantity         int64             `json:"quantity"`
	LineTotal        int64             `json:"line_total"`
	ImageURL         string            `json:"image_url,omitempty"`
}

type OrderAddress struct {
	AddressID    string `json:"address_id"`
	FullName     string `json:"full_name"`
	PhoneNumber  string `json:"phone_number"`
	AddressLine1 string `json:"address_line1"`
	AddressLine2 string `json:"address_line2"`
	Landmark     string `json:"landmark"`
	City         string `json:"city"`
	State        string `json:"state"`
	StateCode    string `json:"state_code"`
	Pincode      string `json:"pincode"`
	Country      string `json:"country"`
}

type StateChange struct {
	FromState string    `json:"from_state"`
	ToState   string    `json:"to_state"`
	Reason    string    `json:"reason"`
	Actor     string    `json:"actor"`
	CreatedAt time.Time `json:"created_at"`
}

type PaymentAction struct {
	Type           string `json:"type"`
	RedirectURL    string `json:"redirect_url,omitempty"`
	QRImageDataURI string `json:"qr_image_data_uri,omitempty"`
	UPIID          string `json:"upi_id,omitempty"`
	UPIIntentURL   string `json:"upi_intent_url,omitempty"`
}

type Payment struct {
	ID                 string         `json:"id"`
	OrderID            string         `json:"order_id"`
	UserID             string         `json:"user_id,omitempty"`
	Provider           string         `json:"provider"`
	State              string         `json:"state"`
	Amount             int64          `json:"amount"`
	Currency           string         `json:"currency"`
	CheckoutURL        string         `json:"checkout_url,omitempty"`
	GatewayReference   string         `json:"gateway_reference,omitempty"`
	GatewayPaymentID   string         `json:"gateway_payment_id,omitempty"`
	ProofTransactionID string         `json:"proof_transaction_id,omitempty"`
	ProofSubmittedAt   *time.Time     `json:"proof_submitted_at,omitempty"`
	HasProofImage      bool           `json:"has_proof_image"`
	FailureReason      string         `json:"failure_reason,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
	Events             []PaymentEvent `json:"events,omitempty"`
	Action             *PaymentAction `json:"action,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type PaymentEvent struct {
	Source    string         `json:"source"`
	Kind      string         `json:"kind"`
	FromState string         `json:"from_state,omitempty"`
	ToState   string         `json:"to_state,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type Order struct {
	ID                 string         `json:"id"`
	UserID             string         `json:"user_id,omitempty"`
	State              string         `json:"state"`
	Currency           string         `json:"currency"`
	Subtotal           int64          `json:"subtotal"`
	ShippingFee        int64          `json:"shipping_fee"`
	Discount           int64          `json:"discount"`
	TotalPayable       int64          `json:"total_payable"`
	ItemCount          int64          `json:"item_count"`
	Items              []OrderItem    `json:"items"`
	ShippingAddress    OrderAddress   `json:"shipping_address"`
	Payment            *Payment       `json:"payment"`
	Tracking           *Tracking      `json:"tracking"`
	Steps              []Step         `json:"steps"`
	Closed             bool           `json:"closed"`
	Payments           []Payment      `json:"payments,omitempty"`
	History            []StateChange  `json:"history,omitempty"`
	AllowedTransitions []string       `json:"allowed_transitions,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
	CanPay             bool           `json:"can_pay"`
	CanCancel          bool           `json:"can_cancel"`
	CanSubmitProof     bool           `json:"can_submit_proof"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type Tracking struct {
	Courier string `json:"courier"`
	Number  string `json:"number"`
	URL     string `json:"url"`
}

type Step struct {
	Key     string     `json:"key"`
	Label   string     `json:"label"`
	Done    bool       `json:"done"`
	Current bool       `json:"current"`
	At      *time.Time `json:"at"`
}

type QuoteLine struct {
	VariantID   string            `json:"variant_id"`
	ProductID   string            `json:"product_id"`
	ProductName string            `json:"product_name"`
	SKU         string            `json:"sku"`
	Attributes  map[string]string `json:"attributes"`
	ImageURL    string            `json:"image_url,omitempty"`
	UnitPrice   int64             `json:"unit_price"`
	Quantity    int64             `json:"quantity"`
	LineTotal   int64             `json:"line_total"`
	Available   bool              `json:"available"`
	StockUnits  uint64            `json:"stock_units"`
}

type Quote struct {
	Lines             []QuoteLine `json:"lines"`
	ItemCount         int64       `json:"item_count"`
	Subtotal          int64       `json:"subtotal"`
	ShippingFee       int64       `json:"shipping_fee"`
	TotalPayable      int64       `json:"total_payable"`
	FreeShippingAbove int64       `json:"free_shipping_above"`
	Currency          string      `json:"currency"`
	Ready             bool        `json:"ready"`
	Problems          []string    `json:"problems"`
	PaymentProvider   string      `json:"payment_provider"`
}

type QuoteRequest struct {
	AccessToken string `json:"access_token"`
}

type QuoteResponse struct {
	Quote Quote `json:"quote"`
}

type PlaceOrderRequest struct {
	AccessToken    string `json:"access_token"`
	AddressID      string `json:"address_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

type OrderResponse struct {
	Order Order `json:"order"`
}

type ListOrdersRequest struct {
	AccessToken string   `json:"access_token"`
	States      []string `json:"states"`
	Cursor      string   `json:"cursor"`
	Limit       int      `json:"limit"`
}

type ListOrdersResponse struct {
	Orders     []Order `json:"orders"`
	NextCursor string  `json:"next_cursor"`
}

type GetOrderRequest struct {
	AccessToken string `json:"access_token"`
	OrderID     string `json:"order_id"`
}

type PayOrderRequest struct {
	AccessToken string `json:"access_token"`
	OrderID     string `json:"order_id"`
}

type SubmitPaymentProofRequest struct {
	AccessToken   string `json:"access_token"`
	OrderID       string `json:"order_id"`
	TransactionID string `json:"transaction_id"`
	Image         []byte `json:"-"`
}

type CancelOrderRequest struct {
	AccessToken string `json:"access_token"`
	OrderID     string `json:"order_id"`
}

type ProviderWebhookRequest struct {
	Form url.Values
}

type ProviderRedirectRequest struct {
	Query url.Values
}

type ProviderCallbackResponse struct {
	OrderID      string `json:"order_id"`
	PaymentID    string `json:"payment_id"`
	PaymentState string `json:"payment_state"`
}

type AdminListOrdersRequest struct {
	States []string `json:"states"`
	Cursor string   `json:"cursor"`
	Limit  int      `json:"limit"`
}

type AdminTransitionOrderRequest struct {
	OrderID string `json:"order_id"`
	To      string `json:"to"`
	Reason  string `json:"reason"`
}

type AdminSetTrackingRequest struct {
	OrderID string `json:"order_id"`
	Courier string `json:"courier"`
	Number  string `json:"number"`
	URL     string `json:"url"`
}

type AdminDecidePaymentRequest struct {
	PaymentID string `json:"payment_id"`
	Status    string `json:"status"`
	Note      string `json:"note"`
}

type AdminListPaymentsRequest struct {
	States   []string `json:"states"`
	Provider string   `json:"provider"`
	Cursor   string   `json:"cursor"`
	Limit    int      `json:"limit"`
}

type ListPaymentsResponse struct {
	Payments   []Payment `json:"payments"`
	NextCursor string    `json:"next_cursor"`
}

type PaymentResponse struct {
	Payment Payment `json:"payment"`
}
