package cartsService

import (
	"context"
	"slices"
	"strings"

	cartsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/carts"
	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
	usersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/users"
)

const (
	maxCartItems       = 50
	maxQuantityPerItem = 10
)

type Service interface {
	GetCart(ctx context.Context, request GetCartRequest) (GetCartResponse, error)
	AddItem(ctx context.Context, request AddItemRequest) (AddItemResponse, error)
	SetItemQuantity(ctx context.Context, request SetItemQuantityRequest) (SetItemQuantityResponse, error)
	RemoveItem(ctx context.Context, request RemoveItemRequest) (RemoveItemResponse, error)
	ClearCart(ctx context.Context, request ClearCartRequest) (ClearCartResponse, error)
	MergeItems(ctx context.Context, request MergeItemsRequest) (MergeItemsResponse, error)
}

type service struct {
	usersSvc     usersService.Service
	productsRepo productsRepository.Repository
	cartsRepo    cartsRepository.Repository
}

func NewService(usersSvc usersService.Service, productsRepo productsRepository.Repository, cartsRepo cartsRepository.Repository) (Service, error) {
	if usersSvc == nil {
		return nil, ErrUsersServiceRequired
	}
	if productsRepo == nil {
		return nil, ErrProductsRepoRequired
	}
	if cartsRepo == nil {
		return nil, ErrCartsRepoRequired
	}
	return &service{usersSvc: usersSvc, productsRepo: productsRepo, cartsRepo: cartsRepo}, nil
}

func (s *service) GetCart(ctx context.Context, request GetCartRequest) (GetCartResponse, error) {
	userID, err := s.authenticate(ctx, request.AccessToken)
	if err != nil {
		return GetCartResponse{}, err
	}
	model, err := s.cartsRepo.GetCart(ctx, userID)
	if err != nil {
		return GetCartResponse{}, err
	}
	cart, err := s.toCart(ctx, model)
	if err != nil {
		return GetCartResponse{}, err
	}
	return GetCartResponse{Cart: cart}, nil
}

func (s *service) AddItem(ctx context.Context, request AddItemRequest) (AddItemResponse, error) {
	userID, err := s.authenticate(ctx, request.AccessToken)
	if err != nil {
		return AddItemResponse{}, err
	}
	if request.Quantity < 1 {
		return AddItemResponse{}, ErrQuantityInvalid
	}
	if request.Quantity > maxQuantityPerItem {
		return AddItemResponse{}, ErrQuantityTooLarge
	}
	variantID := strings.TrimSpace(request.VariantID)
	model, err := s.cartsRepo.UpdateCart(ctx, userID, func(m cartsRepository.Model) (cartsRepository.Model, error) {
		index := findItem(m.Items, variantID)
		quantity := request.Quantity
		if index >= 0 {
			quantity += m.Items[index].Quantity
		} else if len(m.Items) >= maxCartItems {
			return m, ErrCartFull
		}
		if err := s.checkQuantity(ctx, variantID, quantity); err != nil {
			return m, err
		}
		if index >= 0 {
			m.Items[index].Quantity = quantity
			return m, nil
		}
		now, err := s.clock()
		if err != nil {
			return m, err
		}
		m.Items = append(m.Items, cartsRepository.Item{VariantID: variantID, Quantity: quantity, AddedAt: now})
		return m, nil
	})
	if err != nil {
		return AddItemResponse{}, err
	}
	cart, err := s.toCart(ctx, model)
	if err != nil {
		return AddItemResponse{}, err
	}
	return AddItemResponse{Cart: cart}, nil
}

func (s *service) SetItemQuantity(ctx context.Context, request SetItemQuantityRequest) (SetItemQuantityResponse, error) {
	userID, err := s.authenticate(ctx, request.AccessToken)
	if err != nil {
		return SetItemQuantityResponse{}, err
	}
	if request.Quantity < 0 {
		return SetItemQuantityResponse{}, ErrQuantityInvalid
	}
	if request.Quantity > maxQuantityPerItem {
		return SetItemQuantityResponse{}, ErrQuantityTooLarge
	}
	variantID := strings.TrimSpace(request.VariantID)
	model, err := s.cartsRepo.UpdateCart(ctx, userID, func(m cartsRepository.Model) (cartsRepository.Model, error) {
		index := findItem(m.Items, variantID)
		if index < 0 {
			return m, ErrCartItemNotFound
		}
		if request.Quantity == 0 {
			m.Items = slices.Delete(m.Items, index, index+1)
			return m, nil
		}
		if err := s.checkQuantity(ctx, variantID, request.Quantity); err != nil {
			return m, err
		}
		m.Items[index].Quantity = request.Quantity
		return m, nil
	})
	if err != nil {
		return SetItemQuantityResponse{}, err
	}
	cart, err := s.toCart(ctx, model)
	if err != nil {
		return SetItemQuantityResponse{}, err
	}
	return SetItemQuantityResponse{Cart: cart}, nil
}

func (s *service) RemoveItem(ctx context.Context, request RemoveItemRequest) (RemoveItemResponse, error) {
	userID, err := s.authenticate(ctx, request.AccessToken)
	if err != nil {
		return RemoveItemResponse{}, err
	}
	variantID := strings.TrimSpace(request.VariantID)
	model, err := s.cartsRepo.UpdateCart(ctx, userID, func(m cartsRepository.Model) (cartsRepository.Model, error) {
		index := findItem(m.Items, variantID)
		if index < 0 {
			return m, ErrCartItemNotFound
		}
		m.Items = slices.Delete(m.Items, index, index+1)
		return m, nil
	})
	if err != nil {
		return RemoveItemResponse{}, err
	}
	cart, err := s.toCart(ctx, model)
	if err != nil {
		return RemoveItemResponse{}, err
	}
	return RemoveItemResponse{Cart: cart}, nil
}

func (s *service) ClearCart(ctx context.Context, request ClearCartRequest) (ClearCartResponse, error) {
	userID, err := s.authenticate(ctx, request.AccessToken)
	if err != nil {
		return ClearCartResponse{}, err
	}
	if err := s.cartsRepo.DeleteCart(ctx, userID); err != nil {
		return ClearCartResponse{}, err
	}
	return ClearCartResponse{Cart: emptyCart()}, nil
}

func (s *service) MergeItems(ctx context.Context, request MergeItemsRequest) (MergeItemsResponse, error) {
	userID, err := s.authenticate(ctx, request.AccessToken)
	if err != nil {
		return MergeItemsResponse{}, err
	}
	inputs, err := normalizeInputs(request.Items)
	if err != nil {
		return MergeItemsResponse{}, err
	}
	var skipped []string
	model, err := s.cartsRepo.UpdateCart(ctx, userID, func(m cartsRepository.Model) (cartsRepository.Model, error) {
		skipped = skipped[:0]
		now, err := s.clock()
		if err != nil {
			return m, err
		}
		for _, input := range inputs {
			stock, found, err := s.stockOf(ctx, input.VariantID)
			if err != nil {
				return m, err
			}
			if !found || stock == 0 {
				skipped = append(skipped, input.VariantID)
				continue
			}
			index := findItem(m.Items, input.VariantID)
			quantity := input.Quantity
			if index >= 0 {
				quantity += m.Items[index].Quantity
			}
			quantity = capQuantity(quantity, stock)
			if index >= 0 {
				m.Items[index].Quantity = quantity
				continue
			}
			m.Items = append(m.Items, cartsRepository.Item{VariantID: input.VariantID, Quantity: quantity, AddedAt: now})
		}
		if len(m.Items) > maxCartItems {
			return m, ErrCartFull
		}
		return m, nil
	})
	if err != nil {
		return MergeItemsResponse{}, err
	}
	cart, err := s.toCart(ctx, model)
	if err != nil {
		return MergeItemsResponse{}, err
	}
	if skipped == nil {
		skipped = []string{}
	}
	return MergeItemsResponse{Cart: cart, Skipped: skipped}, nil
}
