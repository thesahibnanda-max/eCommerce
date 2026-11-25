package productsRepository

const (
	fieldName        = "name"
	fieldDescription = "description"
	fieldAttributes  = "attributes"
	fieldCategories  = "categories"
	fieldSKUs        = "skus"
	fieldMinPrice    = "min_price"
	fieldMaxPrice    = "max_price"
	fieldInStock     = "in_stock"
	facetCategories  = "categories"
	facetSize        = 20
	nameBoost        = 3
	nameFuzziness    = 1
	wideFuzziness    = 2
	wideTokenLength  = 5
	prefixBoost      = 2
	minTokenLength   = 2
	minSuggestLength = 3
)

type searchDoc struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Attributes  string   `json:"attributes"`
	Categories  []string `json:"categories"`
	SKUs        []string `json:"skus"`
	MinPrice    float64  `json:"min_price"`
	MaxPrice    float64  `json:"max_price"`
	InStock     bool     `json:"in_stock"`
}
