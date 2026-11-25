package productsService

import "time"

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

type Product struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Variants    []Variant      `json:"variants"`
	Categories  []string       `json:"categories,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type CategoryCount struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

type GetProductByIDRequest struct {
	ID string `json:"id"`
}

type GetProductByIDResponse struct {
	Product Product `json:"product"`
}

type GetProductsByIDsRequest struct {
	IDs []string `json:"ids"`
}

type GetProductsByIDsResponse struct {
	Products []Product `json:"products"`
}

type GetProductBySKURequest struct {
	SKU string `json:"sku"`
}

type GetProductBySKUResponse struct {
	Product Product `json:"product"`
	Variant Variant `json:"variant"`
}

type GetProductByVariantIDRequest struct {
	VariantID string `json:"variant_id"`
}

type GetProductByVariantIDResponse struct {
	Product Product `json:"product"`
	Variant Variant `json:"variant"`
}

type ListProductsRequest struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

type ListProductsResponse struct {
	Products []Product `json:"products"`
	Total    int       `json:"total"`
}

type ListProductsByCategoryRequest struct {
	Category string `json:"category"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
}

type ListProductsByCategoryResponse struct {
	Products []Product `json:"products"`
	Total    int       `json:"total"`
}

type ListCategoriesRequest struct{}

type ListCategoriesResponse struct {
	Categories []CategoryCount `json:"categories"`
}

type SearchProductsRequest struct {
	Text        string   `json:"text"`
	Categories  []string `json:"categories,omitempty"`
	MinPrice    *int64   `json:"min_price,omitempty"`
	MaxPrice    *int64   `json:"max_price,omitempty"`
	InStockOnly bool     `json:"in_stock_only"`
	Sort        string   `json:"sort"`
	Offset      int      `json:"offset"`
	Limit       int      `json:"limit"`
}

type SearchProductsResponse struct {
	Products          []Product       `json:"products"`
	Total             int             `json:"total"`
	CategoryFacets    []CategoryCount `json:"category_facets"`
	Suggestion        string          `json:"suggestion"`
	SuggestedProducts []Product       `json:"suggested_products"`
}

type RecommendProductsRequest struct {
	ProductID string `json:"product_id"`
	Limit     int    `json:"limit"`
}

type RecommendProductsResponse struct {
	Products []Product `json:"products"`
}
