package adminService

import "time"

type VariantInput struct {
	ID           string            `json:"id,omitempty"`
	SKU          string            `json:"sku"`
	Attributes   map[string]string `json:"attributes,omitempty"`
	ShownPrice   int64             `json:"shown_price"`
	SellingPrice int64             `json:"selling_price"`
	Currency     string            `json:"currency"`
	StockUnits   uint64            `json:"stock_units"`
	ImageURLs    []string          `json:"image_urls,omitempty"`
}

type ProductInput struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Variants    []VariantInput `json:"variants"`
	Categories  []string       `json:"categories,omitempty"`
}

type ProductUpdateInput struct {
	ID string `json:"id"`
	ProductInput
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

type ImageInput struct {
	FileName string         `json:"file_name"`
	Content  []byte         `json:"content"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type ImageInfo struct {
	ID           string         `json:"id"`
	FileName     string         `json:"file_name"`
	ContentType  string         `json:"content_type"`
	Width        int            `json:"width"`
	Height       int            `json:"height"`
	Size         int            `json:"size"`
	OriginalSize int            `json:"original_size"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type CreateProductRequest struct {
	AdminKey string       `json:"admin_key"`
	Product  ProductInput `json:"product"`
}

type CreateProductResponse struct {
	Product Product `json:"product"`
}

type CreateProductsRequest struct {
	AdminKey string         `json:"admin_key"`
	Products []ProductInput `json:"products"`
}

type CreateProductsResponse struct {
	Products []Product `json:"products"`
}

type UpdateProductRequest struct {
	AdminKey string             `json:"admin_key"`
	Product  ProductUpdateInput `json:"product"`
}

type UpdateProductResponse struct {
	Product Product `json:"product"`
}

type UpdateProductsRequest struct {
	AdminKey string               `json:"admin_key"`
	Products []ProductUpdateInput `json:"products"`
}

type UpdateProductsResponse struct {
	Products []Product `json:"products"`
}

type CreateImageRequest struct {
	AdminKey string     `json:"admin_key"`
	Image    ImageInput `json:"image"`
}

type CreateImageResponse struct {
	Image ImageInfo `json:"image"`
}

type CreateImagesRequest struct {
	AdminKey string       `json:"admin_key"`
	Images   []ImageInput `json:"images"`
}

type CreateImagesResponse struct {
	Images []ImageInfo `json:"images"`
}

type DeleteImageRequest struct {
	AdminKey string `json:"admin_key"`
	ID       string `json:"id"`
}

type DeleteImageResponse struct {
	ID string `json:"id"`
}

type DeleteImagesRequest struct {
	AdminKey string   `json:"admin_key"`
	IDs      []string `json:"ids"`
}

type DeleteImagesResponse struct {
	IDs []string `json:"ids"`
}
