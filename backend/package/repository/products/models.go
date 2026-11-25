package productsRepository

import (
	"time"
)

type Model struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Variants    []Variant      `json:"variants"`
	Categories  []string       `json:"categories,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type Variant struct {
	ID           string            `json:"id"`
	SKU          string            `json:"sku"`
	Attributes   map[string]string `json:"attributes,omitempty"`
	ShownPrice   int64             `json:"shown_price"`
	SellingPrice int64             `json:"selling_price"`
	Currency     string            `json:"currency"`
	StockUnits   uint64            `json:"stock_units"`
	ImageURLs    []string          `json:"image_urls,omitempty"`
}

type Sort int

const (
	SortRelevance Sort = iota
	SortPriceAsc
	SortPriceDesc
	SortNewest
)

type SearchQuery struct {
	Text        string
	Categories  []string
	MinPrice    *int64
	MaxPrice    *int64
	InStockOnly bool
	Sort        Sort
	Offset      int
	Limit       int
	Relaxed     bool
}

type CategoryCount struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

type SearchResult struct {
	Products       []Model         `json:"products"`
	Total          int             `json:"total"`
	CategoryFacets []CategoryCount `json:"category_facets"`
}

type StockChange struct {
	VariantID string `json:"variant_id"`
	Delta     int64  `json:"delta"`
}
