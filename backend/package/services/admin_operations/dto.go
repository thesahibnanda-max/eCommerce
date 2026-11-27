package adminOperationsService

import (
	snapshotService "github.com/thesahibnanda-max/eCommerce/backend/package/services/snapshot"
	"time"

	addressesService "github.com/thesahibnanda-max/eCommerce/backend/package/services/addresses"
	ordersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/orders"
)

type User struct {
	ID                     string         `json:"id"`
	Name                   string         `json:"name"`
	Email                  string         `json:"email"`
	PhoneNumber            string         `json:"phone_number"`
	IsEmailValidated       bool           `json:"is_email_validated"`
	IsPhoneNumberValidated bool           `json:"is_phone_number_validated"`
	Metadata               map[string]any `json:"metadata"`
	CreatedAt              time.Time      `json:"created_at"`
	UpdatedAt              time.Time      `json:"updated_at"`
}

type CartLine struct {
	VariantID    string            `json:"variant_id"`
	ProductID    string            `json:"product_id,omitempty"`
	ProductName  string            `json:"product_name,omitempty"`
	SKU          string            `json:"sku,omitempty"`
	Attributes   map[string]string `json:"attributes,omitempty"`
	SellingPrice int64             `json:"selling_price"`
	StockUnits   uint64            `json:"stock_units"`
	Quantity     int               `json:"quantity"`
	Exists       bool              `json:"exists"`
	AddedAt      time.Time         `json:"added_at"`
}

type Cart struct {
	UserID    string     `json:"user_id"`
	Lines     []CartLine `json:"lines"`
	ItemCount int        `json:"item_count"`
	Value     int64      `json:"value"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type Variant struct {
	ID           string            `json:"id"`
	SKU          string            `json:"sku"`
	Attributes   map[string]string `json:"attributes"`
	ShownPrice   int64             `json:"shown_price"`
	SellingPrice int64             `json:"selling_price"`
	Currency     string            `json:"currency"`
	StockUnits   uint64            `json:"stock_units"`
	ImageURLs    []string          `json:"image_urls"`
}

type Product struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Categories  []string       `json:"categories"`
	Metadata    map[string]any `json:"metadata"`
	Variants    []Variant      `json:"variants"`
	TotalStock  uint64         `json:"total_stock"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type Image struct {
	ID           string         `json:"id"`
	FileName     string         `json:"file_name"`
	ContentType  string         `json:"content_type"`
	Width        int            `json:"width"`
	Height       int            `json:"height"`
	Size         int            `json:"size"`
	OriginalSize int            `json:"original_size"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
}

type Overview struct {
	Users                  int              `json:"users"`
	Products               int              `json:"products"`
	Images                 int              `json:"images"`
	Carts                  int              `json:"carts"`
	OrdersByState          map[string]int64 `json:"orders_by_state"`
	PaymentsByState        map[string]int64 `json:"payments_by_state"`
	PaymentsAwaitingReview int64            `json:"payments_awaiting_review"`
	OrdersToFulfil         int64            `json:"orders_to_fulfil"`
	PaymentProvider        string           `json:"payment_provider"`
}

type KeyRequest struct {
	AdminKey string `json:"admin_key"`
}

type PageRequest struct {
	AdminKey string `json:"admin_key"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
}

type IDRequest struct {
	AdminKey string `json:"admin_key"`
	ID       string `json:"id"`
}

type OverviewResponse struct {
	Overview Overview `json:"overview"`
}

type ListUsersResponse struct {
	Users []User `json:"users"`
	Total int    `json:"total"`
}

type UserDetailResponse struct {
	User      User                       `json:"user"`
	Addresses []addressesService.Address `json:"addresses"`
	Cart      Cart                       `json:"cart"`
	Orders    []ordersService.Order      `json:"orders"`
}

type ListCartsResponse struct {
	Carts []Cart `json:"carts"`
	Total int    `json:"total"`
}

type ListProductsResponse struct {
	Products []Product `json:"products"`
	Total    int       `json:"total"`
}

type ListImagesResponse struct {
	Images []Image `json:"images"`
	Total  int     `json:"total"`
}

type ListOrdersRequest struct {
	AdminKey string   `json:"admin_key"`
	States   []string `json:"states"`
	Cursor   string   `json:"cursor"`
	Limit    int      `json:"limit"`
}

type TransitionOrderRequest struct {
	AdminKey string `json:"admin_key"`
	ID       string `json:"id"`
	To       string `json:"to"`
	Reason   string `json:"reason"`
}

type ListPaymentsRequest struct {
	AdminKey string   `json:"admin_key"`
	States   []string `json:"states"`
	Provider string   `json:"provider"`
	Cursor   string   `json:"cursor"`
	Limit    int      `json:"limit"`
}

type DecidePaymentRequest struct {
	AdminKey string `json:"admin_key"`
	ID       string `json:"id"`
	Status   string `json:"status"`
	Note     string `json:"note"`
}

type ProofImage struct {
	Content     []byte
	ContentType string
}

type SetTrackingRequest struct {
	AdminKey string `json:"admin_key"`
	ID       string `json:"id"`
	Courier  string `json:"courier"`
	Number   string `json:"number"`
	URL      string `json:"url"`
}

type SnapshotResponse struct {
	Configured bool                    `json:"configured"`
	Interval   string                  `json:"interval,omitempty"`
	Running    bool                    `json:"running"`
	Last       *snapshotService.Result `json:"last,omitempty"`
	Result     *snapshotService.Result `json:"result,omitempty"`
}
