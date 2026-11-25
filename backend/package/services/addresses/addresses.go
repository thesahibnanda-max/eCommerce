package addressesService

import (
	"context"
	"strings"

	addressRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/address"
	usersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/users"
)

type Service interface {
	ListAddresses(ctx context.Context, request ListAddressesRequest) (ListAddressesResponse, error)
	CreateAddress(ctx context.Context, request CreateAddressRequest) (AddressResponse, error)
	UpdateAddress(ctx context.Context, request UpdateAddressRequest) (AddressResponse, error)
	DeleteAddress(ctx context.Context, request DeleteAddressRequest) (DeleteAddressResponse, error)
	SetDefaultAddress(ctx context.Context, request SetDefaultAddressRequest) (AddressResponse, error)
}

type service struct {
	usersSvc    usersService.Service
	addressRepo addressRepository.Repository
}

func NewService(usersSvc usersService.Service, addressRepo addressRepository.Repository) (Service, error) {
	if usersSvc == nil {
		return nil, ErrUsersServiceRequired
	}
	if addressRepo == nil {
		return nil, ErrAddressRepoRequired
	}
	return &service{usersSvc: usersSvc, addressRepo: addressRepo}, nil
}

func (s *service) ListAddresses(ctx context.Context, request ListAddressesRequest) (ListAddressesResponse, error) {
	userID, err := s.authenticate(ctx, request.AccessToken)
	if err != nil {
		return ListAddressesResponse{}, err
	}
	models, err := s.addressRepo.ListAddresses(ctx, userID)
	if err != nil {
		return ListAddressesResponse{}, err
	}
	return ListAddressesResponse{Addresses: ToAddresses(models)}, nil
}

func (s *service) CreateAddress(ctx context.Context, request CreateAddressRequest) (AddressResponse, error) {
	userID, err := s.authenticate(ctx, request.AccessToken)
	if err != nil {
		return AddressResponse{}, err
	}
	model := applyInput(addressRepository.Model{UserID: userID, IsDefault: request.Address.IsDefault}, request.Address)
	created, err := s.addressRepo.CreateAddress(ctx, model)
	if err != nil {
		return AddressResponse{}, err
	}
	return AddressResponse{Address: ToAddress(created)}, nil
}

func (s *service) UpdateAddress(ctx context.Context, request UpdateAddressRequest) (AddressResponse, error) {
	userID, err := s.authenticate(ctx, request.AccessToken)
	if err != nil {
		return AddressResponse{}, err
	}
	id := strings.TrimSpace(request.ID)
	updated, err := s.addressRepo.UpdateAddress(ctx, userID, id, func(m addressRepository.Model) (addressRepository.Model, error) {
		return applyInput(m, request.Address), nil
	})
	if err != nil {
		return AddressResponse{}, err
	}
	if request.Address.IsDefault && !updated.IsDefault {
		if updated, err = s.addressRepo.SetDefaultAddress(ctx, userID, id); err != nil {
			return AddressResponse{}, err
		}
	}
	return AddressResponse{Address: ToAddress(updated)}, nil
}

func (s *service) DeleteAddress(ctx context.Context, request DeleteAddressRequest) (DeleteAddressResponse, error) {
	userID, err := s.authenticate(ctx, request.AccessToken)
	if err != nil {
		return DeleteAddressResponse{}, err
	}
	id := strings.TrimSpace(request.ID)
	if err := s.addressRepo.DeleteAddress(ctx, userID, id); err != nil {
		return DeleteAddressResponse{}, err
	}
	return DeleteAddressResponse{ID: id}, nil
}

func (s *service) SetDefaultAddress(ctx context.Context, request SetDefaultAddressRequest) (AddressResponse, error) {
	userID, err := s.authenticate(ctx, request.AccessToken)
	if err != nil {
		return AddressResponse{}, err
	}
	updated, err := s.addressRepo.SetDefaultAddress(ctx, userID, strings.TrimSpace(request.ID))
	if err != nil {
		return AddressResponse{}, err
	}
	return AddressResponse{Address: ToAddress(updated)}, nil
}
