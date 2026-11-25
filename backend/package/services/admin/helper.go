package adminService

import (
	"context"
	"crypto/subtle"
	"strings"

	imagesRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/images"
	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

func (s *service) authorize(ctx context.Context, adminKey string) error {
	if err := utils.CheckContext(ctx); err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(adminKey)), []byte(s.adminKey)) != 1 {
		return ErrUnauthorized
	}
	return nil
}

func (s *service) authorizeBatch(ctx context.Context, adminKey string, size int) error {
	if err := s.authorize(ctx, adminKey); err != nil {
		return err
	}
	return checkBatchSize(size)
}

func checkBatchSize(size int) error {
	if size == 0 {
		return ErrBatchEmpty
	}
	if size > maxBatchSize {
		return ErrBatchTooLarge
	}
	return nil
}

func toVariantModel(in VariantInput) productsRepository.Variant {
	return productsRepository.Variant{
		ID:           in.ID,
		SKU:          in.SKU,
		Attributes:   in.Attributes,
		ShownPrice:   in.ShownPrice,
		SellingPrice: in.SellingPrice,
		Currency:     in.Currency,
		StockUnits:   in.StockUnits,
		ImageURLs:    in.ImageURLs,
	}
}

func toProductModel(id string, in ProductInput) productsRepository.Model {
	variants := make([]productsRepository.Variant, len(in.Variants))
	for i, v := range in.Variants {
		variants[i] = toVariantModel(v)
	}
	return productsRepository.Model{
		ID:          id,
		Name:        in.Name,
		Description: in.Description,
		Metadata:    in.Metadata,
		Variants:    variants,
		Categories:  in.Categories,
	}
}

func toProduct(m productsRepository.Model) Product {
	variants := make([]Variant, len(m.Variants))
	for i, v := range m.Variants {
		variants[i] = Variant{
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

func toImageModel(in ImageInput) imagesRepository.Model {
	return imagesRepository.Model{
		FileName:     in.FileName,
		ImageContent: in.Content,
		Metadata:     in.Metadata,
	}
}

func toImageInfo(m imagesRepository.Model) ImageInfo {
	return ImageInfo{
		ID:           m.ID,
		FileName:     m.FileName,
		ContentType:  m.ContentType,
		Width:        m.Width,
		Height:       m.Height,
		Size:         m.Size,
		OriginalSize: m.OriginalSize,
		Metadata:     m.Metadata,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

func toImageInfos(models []imagesRepository.Model) []ImageInfo {
	out := make([]ImageInfo, len(models))
	for i, m := range models {
		out[i] = toImageInfo(m)
	}
	return out
}
