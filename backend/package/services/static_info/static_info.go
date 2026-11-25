package staticInfoService

import (
	"context"

	staticInfo "github.com/thesahibnanda-max/eCommerce/backend/package/static_info"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

type Service interface {
	ListPages(ctx context.Context, request ListPagesRequest) (ListPagesResponse, error)
	GetPage(ctx context.Context, request GetPageRequest) (GetPageResponse, error)
}

type service struct {
	info staticInfo.StaticInfo
}

func NewService(info staticInfo.StaticInfo) (Service, error) {
	if info == nil {
		return nil, ErrStaticInfoRequired
	}
	return &service{info: info}, nil
}

func (s *service) ListPages(ctx context.Context, _ ListPagesRequest) (ListPagesResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return ListPagesResponse{}, err
	}
	pages, err := s.info.ListPages(ctx)
	if err != nil {
		return ListPagesResponse{}, err
	}
	contact, err := s.info.Contact(ctx)
	if err != nil {
		return ListPagesResponse{}, err
	}
	return ListPagesResponse{Pages: pages, Contact: contact}, nil
}

func (s *service) GetPage(ctx context.Context, request GetPageRequest) (GetPageResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return GetPageResponse{}, err
	}
	page, err := s.info.GetPage(ctx, request.Slug)
	if err != nil {
		return GetPageResponse{}, err
	}
	contact, err := s.info.Contact(ctx)
	if err != nil {
		return GetPageResponse{}, err
	}
	return GetPageResponse{Page: page, Contact: contact}, nil
}
