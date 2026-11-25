package adminOperationsService

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"

	cartsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/carts"
	imagesRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/images"
	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
	usersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/users"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const defaultPageLimit = 20

func (s *service) authorize(ctx context.Context, adminKey string) error {
	if err := utils.CheckContext(ctx); err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(adminKey)), []byte(s.adminKey)) != 1 {
		return ErrUnauthorized
	}
	return nil
}

func pageLimit(limit int) int {
	if limit == 0 {
		return defaultPageLimit
	}
	return limit
}

func toUser(m usersRepository.Model) User {
	return User{
		ID:                     m.ID,
		Name:                   m.Name,
		Email:                  m.Email,
		PhoneNumber:            m.PhoneNumber,
		IsEmailValidated:       m.IsEmailValidated,
		IsPhoneNumberValidated: m.IsPhoneNumberValidated,
		Metadata:               m.Metadata,
		CreatedAt:              m.CreatedAt,
		UpdatedAt:              m.UpdatedAt,
	}
}

func (s *service) toCart(ctx context.Context, m cartsRepository.Model) (Cart, error) {
	cart := Cart{UserID: m.UserID, Lines: make([]CartLine, 0, len(m.Items)), UpdatedAt: m.UpdatedAt}
	for _, item := range m.Items {
		line := CartLine{VariantID: item.VariantID, Quantity: item.Quantity, AddedAt: item.AddedAt}
		product, variant, err := s.products.GetProductByVariantID(ctx, item.VariantID)
		switch {
		case err == nil:
			line.Exists = true
			line.ProductID = product.ID
			line.ProductName = product.Name
			line.SKU = variant.SKU
			line.Attributes = variant.Attributes
			line.SellingPrice = variant.SellingPrice
			line.StockUnits = variant.StockUnits
			cart.Value += variant.SellingPrice * int64(item.Quantity)
		case !errors.Is(err, productsRepository.ErrVariantNotFound) && !errors.Is(err, productsRepository.ErrIDInvalid):
			return Cart{}, err
		}
		cart.ItemCount += item.Quantity
		cart.Lines = append(cart.Lines, line)
	}
	return cart, nil
}

func toProduct(m productsRepository.Model) Product {
	product := Product{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		Categories:  m.Categories,
		Metadata:    m.Metadata,
		Variants:    make([]Variant, 0, len(m.Variants)),
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
	for _, v := range m.Variants {
		product.TotalStock += v.StockUnits
		product.Variants = append(product.Variants, Variant{
			ID:           v.ID,
			SKU:          v.SKU,
			Attributes:   v.Attributes,
			ShownPrice:   v.ShownPrice,
			SellingPrice: v.SellingPrice,
			Currency:     v.Currency,
			StockUnits:   v.StockUnits,
			ImageURLs:    v.ImageURLs,
		})
	}
	return product
}

func toImage(m imagesRepository.Model) Image {
	return Image{
		ID:           m.ID,
		FileName:     m.FileName,
		ContentType:  m.ContentType,
		Width:        m.Width,
		Height:       m.Height,
		Size:         m.Size,
		OriginalSize: m.OriginalSize,
		Metadata:     m.Metadata,
		CreatedAt:    m.CreatedAt,
	}
}
