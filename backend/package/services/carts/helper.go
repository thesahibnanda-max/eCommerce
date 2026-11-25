package cartsService

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	cartsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/carts"
	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
	usersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/users"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

func (s *service) authenticate(ctx context.Context, accessToken string) (string, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return "", err
	}
	user, err := s.usersSvc.VerifyAccessToken(ctx, usersService.VerifyAccessTokenRequest{AccessToken: accessToken})
	if err != nil {
		return "", err
	}
	return user.ID, nil
}

func (s *service) clock() (time.Time, error) {
	return utils.TimeNow()
}

func (s *service) checkQuantity(ctx context.Context, variantID string, quantity int) error {
	if quantity > maxQuantityPerItem {
		return ErrQuantityTooLarge
	}
	if !utils.ValidateULID(variantID) {
		return ErrVariantIDInvalid
	}
	_, variant, err := s.productsRepo.GetProductByVariantID(ctx, variantID)
	if err != nil {
		return err
	}
	if uint64(quantity) > variant.StockUnits {
		return ErrOutOfStock
	}
	return nil
}

func (s *service) stockOf(ctx context.Context, variantID string) (uint64, bool, error) {
	_, variant, err := s.productsRepo.GetProductByVariantID(ctx, variantID)
	if errors.Is(err, productsRepository.ErrVariantNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return variant.StockUnits, true, nil
}

func (s *service) toCart(ctx context.Context, model cartsRepository.Model) (Cart, error) {
	cart := emptyCart()
	cart.UpdatedAt = model.UpdatedAt
	totals := map[string]*Total{}
	for _, item := range model.Items {
		line := CartItem{VariantID: item.VariantID, Quantity: item.Quantity, AddedAt: item.AddedAt}
		product, variant, err := s.productsRepo.GetProductByVariantID(ctx, item.VariantID)
		if err != nil && !errors.Is(err, productsRepository.ErrVariantNotFound) {
			return Cart{}, err
		}
		if err == nil {
			line.ProductID = product.ID
			line.Name = product.Name
			line.SKU = variant.SKU
			line.Attributes = variant.Attributes
			line.ShownPrice = variant.ShownPrice
			line.SellingPrice = variant.SellingPrice
			line.Currency = variant.Currency
			line.StockUnits = variant.StockUnits
			line.Available = variant.StockUnits >= uint64(item.Quantity)
			if len(variant.ImageURLs) > 0 {
				line.ImageURL = variant.ImageURLs[0]
			}
		}
		cart.Items = append(cart.Items, line)
		cart.ItemCount += item.Quantity
		if !line.Available {
			continue
		}
		total, ok := totals[line.Currency]
		if !ok {
			total = &Total{Currency: line.Currency}
			totals[line.Currency] = total
		}
		total.ShownPrice += line.ShownPrice * int64(item.Quantity)
		total.SellingPrice += line.SellingPrice * int64(item.Quantity)
	}
	for _, total := range totals {
		cart.Totals = append(cart.Totals, *total)
	}
	sort.Slice(cart.Totals, func(i, j int) bool { return cart.Totals[i].Currency < cart.Totals[j].Currency })
	return cart, nil
}

func emptyCart() Cart {
	return Cart{Items: []CartItem{}, Totals: []Total{}}
}

func findItem(items []cartsRepository.Item, variantID string) int {
	for i, item := range items {
		if item.VariantID == variantID {
			return i
		}
	}
	return -1
}

func normalizeInputs(items []ItemInput) ([]ItemInput, error) {
	if len(items) == 0 {
		return nil, ErrBatchEmpty
	}
	if len(items) > maxCartItems {
		return nil, ErrTooManyItems
	}
	merged := make([]ItemInput, 0, len(items))
	index := make(map[string]int, len(items))
	for _, item := range items {
		variantID := strings.TrimSpace(item.VariantID)
		if !utils.ValidateULID(variantID) {
			return nil, ErrVariantIDInvalid
		}
		if item.Quantity < 1 {
			return nil, ErrQuantityInvalid
		}
		quantity := min(item.Quantity, maxQuantityPerItem)
		if i, ok := index[variantID]; ok {
			merged[i].Quantity = min(merged[i].Quantity+quantity, maxQuantityPerItem)
			continue
		}
		index[variantID] = len(merged)
		merged = append(merged, ItemInput{VariantID: variantID, Quantity: quantity})
	}
	return merged, nil
}

func capQuantity(quantity int, stock uint64) int {
	limit := maxQuantityPerItem
	if stock < uint64(limit) {
		limit = int(stock)
	}
	if quantity > limit {
		return limit
	}
	return quantity
}
