package ordersRepository

import (
	"time"

	stateMachineUtils "github.com/thesahibnanda-max/eCommerce/backend/package/utils/state_machine"
)

type Model struct {
	ID              string                                     `json:"id"`
	UserID          string                                     `json:"user_id"`
	IdempotencyKey  string                                     `json:"idempotency_key"`
	OrderState      stateMachineUtils.StateMachine[OrderState] `json:"order_state"`
	Currency        Currency                                   `json:"currency"`
	Subtotal        int64                                      `json:"subtotal"`
	ShippingFee     int64                                      `json:"shipping_fee"`
	Discount        int64                                      `json:"discount"`
	TotalPayable    int64                                      `json:"total_payable"`
	ShippingAddress ShippingAddress                            `json:"shipping_address"`
	PaymentGateway  string                                     `json:"payment_gateway"`
	Items           []Item                                     `json:"items"`
	StateHistory    []StateChange                              `json:"state_history"`
	PaymentEvents   []PaymentEvent                             `json:"payment_events"`
	Metadata        map[string]any                             `json:"metadata"`
	CreatedAt       time.Time                                  `json:"created_at"`
	UpdatedAt       time.Time                                  `json:"updated_at"`
}

type Item struct {
	ProductID        string            `json:"product_id"`
	ProductVariantID string            `json:"product_variant_id"`
	SKU              string            `json:"sku"`
	ProductName      string            `json:"product_name"`
	Attributes       map[string]string `json:"attributes"`
	UnitPrice        int64             `json:"unit_price"`
	Quantity         int64             `json:"quantity"`
	LineTotal        int64             `json:"line_total"`
}

type ShippingAddress struct {
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
	FromState OrderState `json:"from_state"`
	ToState   OrderState `json:"to_state"`
	Reason    string     `json:"reason"`
	Actor     Actor      `json:"actor"`
	CreatedAt time.Time  `json:"created_at"`
}

type PaymentEvent struct {
	Gateway          string         `json:"gateway"`
	GatewayEventID   string         `json:"gateway_event_id"`
	EventType        string         `json:"event_type"`
	GatewayOrderID   string         `json:"gateway_order_id"`
	GatewayPaymentID string         `json:"gateway_payment_id"`
	Status           string         `json:"status"`
	Amount           int64          `json:"amount"`
	Raw              map[string]any `json:"raw"`
	CreatedAt        time.Time      `json:"created_at"`
}

type CreateOrderInput struct {
	UserID          string
	IdempotencyKey  string
	Currency        Currency
	ShippingFee     int64
	Discount        int64
	ShippingAddress ShippingAddress
	Items           []Item
	Metadata        map[string]any
}

type ListQuery struct {
	States []OrderState
	Limit  int
	Cursor string
}

type Page struct {
	Orders     []Model `json:"orders"`
	NextCursor string  `json:"next_cursor"`
}

type cursor struct {
	CreatedAt int64  `json:"t"`
	ID        string `json:"i"`
}
