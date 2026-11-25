package adminService

import (
	"context"
	"strings"

	imagesRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/images"
	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
)

const maxBatchSize = 100

type Service interface {
	CreateProduct(ctx context.Context, request CreateProductRequest) (CreateProductResponse, error)
	CreateProducts(ctx context.Context, request CreateProductsRequest) (CreateProductsResponse, error)
	UpdateProduct(ctx context.Context, request UpdateProductRequest) (UpdateProductResponse, error)
	UpdateProducts(ctx context.Context, request UpdateProductsRequest) (UpdateProductsResponse, error)
	CreateImage(ctx context.Context, request CreateImageRequest) (CreateImageResponse, error)
	CreateImages(ctx context.Context, request CreateImagesRequest) (CreateImagesResponse, error)
	DeleteImage(ctx context.Context, request DeleteImageRequest) (DeleteImageResponse, error)
	DeleteImages(ctx context.Context, request DeleteImagesRequest) (DeleteImagesResponse, error)
}

type service struct {
	adminKey     string
	productsRepo productsRepository.Repository
	imagesRepo   imagesRepository.Repository
}

func NewService(adminKey string, productsRepo productsRepository.Repository, imagesRepo imagesRepository.Repository) (Service, error) {
	adminKey = strings.TrimSpace(adminKey)
	if adminKey == "" {
		return nil, ErrAdminKeyRequired
	}
	if productsRepo == nil {
		return nil, ErrProductsRepoRequired
	}
	if imagesRepo == nil {
		return nil, ErrImagesRepoRequired
	}
	return &service{
		adminKey:     adminKey,
		productsRepo: productsRepo,
		imagesRepo:   imagesRepo,
	}, nil
}

func (s *service) CreateProduct(ctx context.Context, request CreateProductRequest) (CreateProductResponse, error) {
	if err := s.authorize(ctx, request.AdminKey); err != nil {
		return CreateProductResponse{}, err
	}
	created, err := s.productsRepo.CreateProduct(ctx, toProductModel("", request.Product))
	if err != nil {
		return CreateProductResponse{}, err
	}
	return CreateProductResponse{Product: toProduct(created)}, nil
}

func (s *service) CreateProducts(ctx context.Context, request CreateProductsRequest) (CreateProductsResponse, error) {
	if err := s.authorizeBatch(ctx, request.AdminKey, len(request.Products)); err != nil {
		return CreateProductsResponse{}, err
	}
	models := make([]productsRepository.Model, len(request.Products))
	for i, p := range request.Products {
		models[i] = toProductModel("", p)
	}
	created, err := s.productsRepo.CreateProducts(ctx, models)
	if err != nil {
		return CreateProductsResponse{}, err
	}
	return CreateProductsResponse{Products: toProducts(created)}, nil
}

func (s *service) UpdateProduct(ctx context.Context, request UpdateProductRequest) (UpdateProductResponse, error) {
	if err := s.authorize(ctx, request.AdminKey); err != nil {
		return UpdateProductResponse{}, err
	}
	updated, err := s.productsRepo.UpdateProduct(ctx, toProductModel(strings.TrimSpace(request.Product.ID), request.Product.ProductInput))
	if err != nil {
		return UpdateProductResponse{}, err
	}
	return UpdateProductResponse{Product: toProduct(updated)}, nil
}

func (s *service) UpdateProducts(ctx context.Context, request UpdateProductsRequest) (UpdateProductsResponse, error) {
	if err := s.authorizeBatch(ctx, request.AdminKey, len(request.Products)); err != nil {
		return UpdateProductsResponse{}, err
	}
	models := make([]productsRepository.Model, len(request.Products))
	for i, p := range request.Products {
		models[i] = toProductModel(strings.TrimSpace(p.ID), p.ProductInput)
	}
	updated, err := s.productsRepo.UpdateProducts(ctx, models)
	if err != nil {
		return UpdateProductsResponse{}, err
	}
	return UpdateProductsResponse{Products: toProducts(updated)}, nil
}

func (s *service) CreateImage(ctx context.Context, request CreateImageRequest) (CreateImageResponse, error) {
	if err := s.authorize(ctx, request.AdminKey); err != nil {
		return CreateImageResponse{}, err
	}
	created, err := s.imagesRepo.CreateImage(ctx, toImageModel(request.Image))
	if err != nil {
		return CreateImageResponse{}, err
	}
	return CreateImageResponse{Image: toImageInfo(created)}, nil
}

func (s *service) CreateImages(ctx context.Context, request CreateImagesRequest) (CreateImagesResponse, error) {
	if err := s.authorizeBatch(ctx, request.AdminKey, len(request.Images)); err != nil {
		return CreateImagesResponse{}, err
	}
	models := make([]imagesRepository.Model, len(request.Images))
	for i, img := range request.Images {
		models[i] = toImageModel(img)
	}
	created, err := s.imagesRepo.CreateImages(ctx, models)
	if err != nil {
		return CreateImagesResponse{}, err
	}
	return CreateImagesResponse{Images: toImageInfos(created)}, nil
}

func (s *service) DeleteImage(ctx context.Context, request DeleteImageRequest) (DeleteImageResponse, error) {
	if err := s.authorize(ctx, request.AdminKey); err != nil {
		return DeleteImageResponse{}, err
	}
	id := strings.TrimSpace(request.ID)
	if err := s.imagesRepo.DeleteImage(ctx, id); err != nil {
		return DeleteImageResponse{}, err
	}
	return DeleteImageResponse{ID: id}, nil
}

func (s *service) DeleteImages(ctx context.Context, request DeleteImagesRequest) (DeleteImagesResponse, error) {
	if err := s.authorizeBatch(ctx, request.AdminKey, len(request.IDs)); err != nil {
		return DeleteImagesResponse{}, err
	}
	ids := make([]string, len(request.IDs))
	for i, id := range request.IDs {
		ids[i] = strings.TrimSpace(id)
	}
	if err := s.imagesRepo.DeleteImages(ctx, ids); err != nil {
		return DeleteImagesResponse{}, err
	}
	return DeleteImagesResponse{IDs: ids}, nil
}
