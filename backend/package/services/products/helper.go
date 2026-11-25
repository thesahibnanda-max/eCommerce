package productsService

import (
	"context"
	"strings"

	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
)

var sorts = map[string]productsRepository.Sort{
	"":           productsRepository.SortRelevance,
	"relevance":  productsRepository.SortRelevance,
	"price_asc":  productsRepository.SortPriceAsc,
	"price_desc": productsRepository.SortPriceDesc,
	"newest":     productsRepository.SortNewest,
}

func parseSort(sort string) (productsRepository.Sort, error) {
	v, ok := sorts[strings.ToLower(strings.TrimSpace(sort))]
	if !ok {
		return 0, ErrSortInvalid
	}
	return v, nil
}

func trimAll(ids []string) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = strings.TrimSpace(id)
	}
	return out
}

func toVariant(v productsRepository.Variant) Variant {
	return Variant{
		ID:           v.ID,
		SKU:          v.SKU,
		Attributes:   v.Attributes,
		ShownPrice:   v.ShownPrice,
		SellingPrice: v.SellingPrice,
		Currency:     v.Currency,
		StockUnits:   v.StockUnits,
		ImageURLs:    v.ImageURLs,
	}
}

func toProduct(m productsRepository.Model) Product {
	variants := make([]Variant, len(m.Variants))
	for i, v := range m.Variants {
		variants[i] = toVariant(v)
	}
	return Product{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		Metadata:    m.Metadata,
		Variants:    variants,
		Categories:  m.Categories,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func toProducts(models []productsRepository.Model) []Product {
	out := make([]Product, len(models))
	for i, m := range models {
		out[i] = toProduct(m)
	}
	return out
}

func toCategoryCounts(counts []productsRepository.CategoryCount) []CategoryCount {
	out := make([]CategoryCount, len(counts))
	for i, c := range counts {
		out[i] = CategoryCount{Category: c.Category, Count: c.Count}
	}
	return out
}

const suggestedLimit = 8

func (s *service) suggest(ctx context.Context, text string) (string, []productsRepository.Model, error) {
	suggestion, err := s.productsRepo.SuggestQuery(ctx, text)
	if err != nil {
		return "", nil, err
	}
	attempts := []productsRepository.SearchQuery{
		{Text: text, Relaxed: true, InStockOnly: true, Limit: suggestedLimit},
		{InStockOnly: true, Sort: productsRepository.SortNewest, Limit: suggestedLimit},
	}
	if suggestion != "" {
		attempts = append([]productsRepository.SearchQuery{{Text: suggestion, InStockOnly: true, Limit: suggestedLimit}}, attempts...)
	}
	for _, attempt := range attempts {
		res, err := s.productsRepo.SearchProducts(ctx, attempt)
		if err != nil {
			return "", nil, err
		}
		if len(res.Products) > 0 {
			return suggestion, res.Products, nil
		}
	}
	return suggestion, []productsRepository.Model{}, nil
}
