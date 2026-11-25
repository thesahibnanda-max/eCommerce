package imagesService

import (
	"context"
	"strings"

	imagesRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/images"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const maxIDs = 100

type Service interface {
	GetImageByID(ctx context.Context, request GetImageByIDRequest) (GetImageByIDResponse, error)
	GetImageInfoByID(ctx context.Context, request GetImageInfoByIDRequest) (GetImageInfoByIDResponse, error)
	GetImagesInfoByIDs(ctx context.Context, request GetImagesInfoByIDsRequest) (GetImagesInfoByIDsResponse, error)
}

type service struct {
	imagesRepo imagesRepository.Repository
}

func NewService(imagesRepo imagesRepository.Repository) (Service, error) {
	if imagesRepo == nil {
		return nil, ErrImagesRepoRequired
	}
	return &service{
		imagesRepo: imagesRepo,
	}, nil
}

func (s *service) GetImageByID(ctx context.Context, request GetImageByIDRequest) (GetImageByIDResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return GetImageByIDResponse{}, err
	}
	img, err := s.imagesRepo.GetImageByID(ctx, strings.TrimSpace(request.ID))
	if err != nil {
		return GetImageByIDResponse{}, err
	}
	return GetImageByIDResponse{Image: Image{ImageInfo: toImageInfo(img), Content: img.ImageContent}}, nil
}

func (s *service) GetImageInfoByID(ctx context.Context, request GetImageInfoByIDRequest) (GetImageInfoByIDResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return GetImageInfoByIDResponse{}, err
	}
	img, err := s.imagesRepo.GetImageInfoByID(ctx, strings.TrimSpace(request.ID))
	if err != nil {
		return GetImageInfoByIDResponse{}, err
	}
	return GetImageInfoByIDResponse{Image: toImageInfo(img)}, nil
}

func (s *service) GetImagesInfoByIDs(ctx context.Context, request GetImagesInfoByIDsRequest) (GetImagesInfoByIDsResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return GetImagesInfoByIDsResponse{}, err
	}
	ids, err := normalizeIDs(request.IDs)
	if err != nil {
		return GetImagesInfoByIDsResponse{}, err
	}
	out := make([]ImageInfo, len(ids))
	for i, id := range ids {
		img, err := s.imagesRepo.GetImageInfoByID(ctx, id)
		if err != nil {
			return GetImagesInfoByIDsResponse{}, err
		}
		out[i] = toImageInfo(img)
	}
	return GetImagesInfoByIDsResponse{Images: out}, nil
}
