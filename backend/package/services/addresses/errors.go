package addressesService

import (
	addressRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/address"
	usersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/users"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	ErrInvalidContext       = utils.ErrContextInvalid
	ErrUsersServiceRequired = Error("addressesService: users service is required")
	ErrAddressRepoRequired  = Error("addressesService: address repository is required")
	ErrAddressNotFound      = addressRepository.ErrAddressNotFound
	ErrAddressIDInvalid     = addressRepository.ErrAddressIDInvalid
	ErrTokenInvalid         = usersService.ErrTokenInvalid
)

type Error string

func (e Error) Error() string {
	return string(e)
}
