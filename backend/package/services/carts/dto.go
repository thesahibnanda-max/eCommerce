package cartsService

import (
	"time"
)

type CartItem struct {
	VariantID    string            `json:"variant_id"`
	ProductID    string            `json:"product_id,omitempty"`
	Name         string            `json:"name,omitempty"`
	SKU          string            `json:"sku,omitempty"`
	Attributes   map[string]string `json:"attributes,omitempty"`
	ImageURL     string            `json:"image_url,omitempty"`
	ShownPrice   int64             `json:"shown_price"`
	SellingPrice int64             `json:"selling_price"`
	Currency     string            `json:"currency,omitempty"`
	StockUnits   uint64            `json:"stock_units"`
	Quantity     int               `json:"quantity"`
	Available    bool              `json:"available"`
	AddedAt      time.Time         `json:"added_at"`
}

type Total struct {
	Currency     string `json:"currency"`
	ShownPrice   int64  `json:"shown_price"`
	SellingPrice int64  `json:"selling_price"`
}

type Cart struct {
	Items     []CartItem `json:"items"`
	ItemCount int        `json:"item_count"`
	Totals    []Total    `json:"totals"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type ItemInput struct {
	VariantID string `json:"variant_id"`
	Quantity  int    `json:"quantity"`
}

type GetCartRequest struct {
	AccessToken string `json:"access_token"`
}

type GetCartResponse struct {
	Cart Cart `json:"cart"`
}

type AddItemRequest struct {
	AccessToken string `json:"access_token"`
	VariantID   string `json:"variant_id"`
	Quantity    int    `json:"quantity"`
}

type AddItemResponse struct {
	Cart Cart `json:"cart"`
}

type SetItemQuantityRequest struct {
	AccessToken string `json:"access_token"`
	VariantID   string `json:"variant_id"`
	Quantity    int    `json:"quantity"`
}

type SetItemQuantityResponse struct {
	Cart Cart `json:"cart"`
}

type RemoveItemRequest struct {
	AccessToken string `json:"access_token"`
	VariantID   string `json:"variant_id"`
}

type RemoveItemResponse struct {
	Cart Cart `json:"cart"`
}

type ClearCartRequest struct {
	AccessToken string `json:"access_token"`
}

type ClearCartResponse struct {
	Cart Cart `json:"cart"`
}

type MergeItemsRequest struct {
	AccessToken string      `json:"access_token"`
	Items       []ItemInput `json:"items"`
}

type MergeItemsResponse struct {
	Cart    Cart     `json:"cart"`
	Skipped []string `json:"skipped"`
}
