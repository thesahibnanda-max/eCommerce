package addressesService

import (
	"context"
	"strings"

	addressRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/address"
	usersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/users"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const countryIndia = "India"

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

func applyInput(m addressRepository.Model, in AddressInput) addressRepository.Model {
	m.Label = in.Label
	m.FullName = in.FullName
	m.PhoneNumber = in.PhoneNumber
	m.AddressLine1 = in.AddressLine1
	m.AddressLine2 = in.AddressLine2
	m.Landmark = in.Landmark
	m.City = in.City
	m.Pincode = strings.TrimSpace(in.Pincode)
	m.Country = countryIndia
	m.IsIndian = true
	return m
}

func ToAddress(m addressRepository.Model) Address {
	return Address{
		ID:           m.ID,
		Label:        m.Label,
		FullName:     m.FullName,
		PhoneNumber:  m.PhoneNumber,
		AddressLine1: m.AddressLine1,
		AddressLine2: m.AddressLine2,
		Landmark:     m.Landmark,
		City:         m.City,
		State:        m.State,
		StateCode:    m.StateCode,
		Country:      m.Country,
		Pincode:      m.Pincode,
		IsDefault:    m.IsDefault,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

func ToAddresses(models []addressRepository.Model) []Address {
	out := make([]Address, 0, len(models))
	for _, m := range models {
		out = append(out, ToAddress(m))
	}
	return out
}
