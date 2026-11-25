package productsService

import (
	"context"
	"strings"

	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const maxIDs = 100

type Service interface {
	GetProductByID(ctx context.Context, request GetProductByIDRequest) (GetProductByIDResponse, error)
	GetProductsByIDs(ctx context.Context, request GetProductsByIDsRequest) (GetProductsByIDsResponse, error)
	GetProductBySKU(ctx context.Context, request GetProductBySKURequest) (GetProductBySKUResponse, error)
	GetProductByVariantID(ctx context.Context, request GetProductByVariantIDRequest) (GetProductByVariantIDResponse, error)
	ListProducts(ctx context.Context, request ListProductsRequest) (ListProductsResponse, error)
	ListProductsByCategory(ctx context.Context, request ListProductsByCategoryRequest) (ListProductsByCategoryResponse, error)
	ListCategories(ctx context.Context, request ListCategoriesRequest) (ListCategoriesResponse, error)
	SearchProducts(ctx context.Context, request SearchProductsRequest) (SearchProductsResponse, error)
	RecommendProducts(ctx context.Context, request RecommendProductsRequest) (RecommendProductsResponse, error)
}

type service struct {
	productsRepo productsRepository.Repository
}

func NewService(productsRepo productsRepository.Repository) (Service, error) {
	if productsRepo == nil {
		return nil, ErrProductsRepoRequired
	}
	return &service{
		productsRepo: productsRepo,
	}, nil
}

func (s *service) GetProductByID(ctx context.Context, request GetProductByIDRequest) (GetProductByIDResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return GetProductByIDResponse{}, err
	}
	p, err := s.productsRepo.GetProductByID(ctx, strings.TrimSpace(request.ID))
	if err != nil {
		return GetProductByIDResponse{}, err
	}
	return GetProductByIDResponse{Product: toProduct(p)}, nil
}

func (s *service) GetProductsByIDs(ctx context.Context, request GetProductsByIDsRequest) (GetProductsByIDsResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return GetProductsByIDsResponse{}, err
	}
	if len(request.IDs) == 0 {
		return GetProductsByIDsResponse{}, ErrBatchEmpty
	}
	if len(request.IDs) > maxIDs {
		return GetProductsByIDsResponse{}, ErrTooManyIDs
	}
	products, err := s.productsRepo.GetProductsByIDs(ctx, trimAll(request.IDs))
	if err != nil {
		return GetProductsByIDsResponse{}, err
	}
	return GetProductsByIDsResponse{Products: toProducts(products)}, nil
}

func (s *service) GetProductBySKU(ctx context.Context, request GetProductBySKURequest) (GetProductBySKUResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return GetProductBySKUResponse{}, err
	}
	p, v, err := s.productsRepo.GetProductBySKU(ctx, strings.TrimSpace(request.SKU))
	if err != nil {
		return GetProductBySKUResponse{}, err
	}
	return GetProductBySKUResponse{Product: toProduct(p), Variant: toVariant(v)}, nil
}

func (s *service) GetProductByVariantID(ctx context.Context, request GetProductByVariantIDRequest) (GetProductByVariantIDResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return GetProductByVariantIDResponse{}, err
	}
	p, v, err := s.productsRepo.GetProductByVariantID(ctx, strings.TrimSpace(request.VariantID))
	if err != nil {
		return GetProductByVariantIDResponse{}, err
	}
	return GetProductByVariantIDResponse{Product: toProduct(p), Variant: toVariant(v)}, nil
}

func (s *service) ListProducts(ctx context.Context, request ListProductsRequest) (ListProductsResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return ListProductsResponse{}, err
	}
	products, total, err := s.productsRepo.ListProducts(ctx, request.Offset, request.Limit)
	if err != nil {
		return ListProductsResponse{}, err
	}
	return ListProductsResponse{Products: toProducts(products), Total: total}, nil
}

func (s *service) ListProductsByCategory(ctx context.Context, request ListProductsByCategoryRequest) (ListProductsByCategoryResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return ListProductsByCategoryResponse{}, err
	}
	products, total, err := s.productsRepo.ListProductsByCategory(ctx, request.Category, request.Offset, request.Limit)
	if err != nil {
		return ListProductsByCategoryResponse{}, err
	}
	return ListProductsByCategoryResponse{Products: toProducts(products), Total: total}, nil
}

func (s *service) ListCategories(ctx context.Context, request ListCategoriesRequest) (ListCategoriesResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return ListCategoriesResponse{}, err
	}
	counts, err := s.productsRepo.ListCategories(ctx)
	if err != nil {
		return ListCategoriesResponse{}, err
	}
	return ListCategoriesResponse{Categories: toCategoryCounts(counts)}, nil
}

func (s *service) SearchProducts(ctx context.Context, request SearchProductsRequest) (SearchProductsResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return SearchProductsResponse{}, err
	}
	sort, err := parseSort(request.Sort)
	if err != nil {
		return SearchProductsResponse{}, err
	}
	res, err := s.productsRepo.SearchProducts(ctx, productsRepository.SearchQuery{
		Text:        request.Text,
		Categories:  request.Categories,
		MinPrice:    request.MinPrice,
		MaxPrice:    request.MaxPrice,
		InStockOnly: request.InStockOnly,
		Sort:        sort,
		Offset:      request.Offset,
		Limit:       request.Limit,
	})
	if err != nil {
		return SearchProductsResponse{}, err
	}
	response := SearchProductsResponse{
		Products:          toProducts(res.Products),
		Total:             res.Total,
		CategoryFacets:    toCategoryCounts(res.CategoryFacets),
		SuggestedProducts: []Product{},
	}
	if res.Total == 0 && request.Offset == 0 && strings.TrimSpace(request.Text) != "" {
		suggestion, suggested, err := s.suggest(ctx, request.Text)
		if err != nil {
			return SearchProductsResponse{}, err
		}
		response.Suggestion = suggestion
		response.SuggestedProducts = toProducts(suggested)
	}
	return response, nil
}

func (s *service) RecommendProducts(ctx context.Context, request RecommendProductsRequest) (RecommendProductsResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return RecommendProductsResponse{}, err
	}
	products, err := s.productsRepo.RecommendProducts(ctx, strings.TrimSpace(request.ProductID), request.Limit)
	if err != nil {
		return RecommendProductsResponse{}, err
	}
	return RecommendProductsResponse{Products: toProducts(products)}, nil
}
