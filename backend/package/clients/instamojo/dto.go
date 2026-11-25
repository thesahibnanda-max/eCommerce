package instamojoClient

import (
	"time"
)

type CreatePaymentRequestRequest struct {
	Amount                int64
	Purpose               string
	BuyerName             string
	Email                 string
	Phone                 string
	RedirectURL           string
	WebhookURL            string
	AllowRepeatedPayments bool
	SendEmail             bool
	ExpiresAt             time.Time
}

type PaymentRequest struct {
	ID                    string               `json:"id"`
	User                  string               `json:"user"`
	Phone                 string               `json:"phone"`
	Email                 string               `json:"email"`
	BuyerName             string               `json:"buyer_name"`
	Amount                string               `json:"amount"`
	Purpose               string               `json:"purpose"`
	Status                PaymentRequestStatus `json:"status"`
	Payments              []string             `json:"payments"`
	SendSMS               bool                 `json:"send_sms"`
	SendEmail             bool                 `json:"send_email"`
	SMSStatus             string               `json:"sms_status"`
	EmailStatus           string               `json:"email_status"`
	ShortURL              string               `json:"shorturl"`
	LongURL               string               `json:"longurl"`
	RedirectURL           string               `json:"redirect_url"`
	Webhook               string               `json:"webhook"`
	ScheduledAt           *time.Time           `json:"scheduled_at"`
	ExpiresAt             *time.Time           `json:"expires_at"`
	AllowRepeatedPayments bool                 `json:"allow_repeated_payments"`
	MarkFulfilled         bool                 `json:"mark_fulfilled"`
	CreatedAt             time.Time            `json:"created_at"`
	ModifiedAt            time.Time            `json:"modified_at"`
	ResourceURI           string               `json:"resource_uri"`
}

type Payment struct {
	ID                  string          `json:"id"`
	Title               string          `json:"title"`
	PaymentType         string          `json:"payment_type"`
	PaymentRequest      string          `json:"payment_request"`
	Status              *bool           `json:"status"`
	Link                string          `json:"link"`
	Product             string          `json:"product"`
	Seller              string          `json:"seller"`
	Currency            string          `json:"currency"`
	Amount              string          `json:"amount"`
	Name                string          `json:"name"`
	Email               string          `json:"email"`
	Phone               string          `json:"phone"`
	Payout              string          `json:"payout"`
	Fees                string          `json:"fees"`
	TotalTaxes          string          `json:"total_taxes"`
	Cases               []any           `json:"cases"`
	AffiliateID         string          `json:"affiliate_id"`
	AffiliateCommission string          `json:"affiliate_commission"`
	OrderInfo           PaymentOrder    `json:"order_info"`
	InstrumentType      InstrumentType  `json:"instrument_type"`
	BillingInstrument   string          `json:"billing_instrument"`
	Failure             *PaymentFailure `json:"failure"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
	TaxInvoiceID        string          `json:"tax_invoice_id"`
	ResourceURI         string          `json:"resource_uri"`
}

type PaymentOrder struct {
	ShippingAddress string         `json:"shipping_address"`
	ShippingCity    string         `json:"shipping_city"`
	ShippingState   string         `json:"shipping_state"`
	ShippingZip     string         `json:"shipping_zip"`
	ShippingCountry string         `json:"shipping_country"`
	Quantity        int64          `json:"quantity"`
	UnitPrice       string         `json:"unit_price"`
	CustomFields    map[string]any `json:"custom_fields"`
	Variants        []any          `json:"variants"`
}

type PaymentFailure struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	Scope       string `json:"scope"`
}
