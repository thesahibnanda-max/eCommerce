package staticInfoService

import (
	staticInfo "github.com/thesahibnanda-max/eCommerce/backend/package/static_info"
)

type ListPagesRequest struct{}

type ListPagesResponse struct {
	Pages   []staticInfo.PageSummary `json:"pages"`
	Contact staticInfo.Contact       `json:"contact"`
}

type GetPageRequest struct {
	Slug string `json:"slug"`
}

type GetPageResponse struct {
	Page    staticInfo.Page    `json:"page"`
	Contact staticInfo.Contact `json:"contact"`
}
