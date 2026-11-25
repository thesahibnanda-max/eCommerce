package productsRepository

type variantRef struct {
	productID string
	variantID string
}

type entry struct {
	product  Model
	minPrice int64
	maxPrice int64
	inStock  bool
}

type cache struct {
	byID        map[string]*entry
	bySKU       map[string]variantRef
	byVariantID map[string]variantRef
	allIDs      []string
	byCategory  map[string][]string
}
