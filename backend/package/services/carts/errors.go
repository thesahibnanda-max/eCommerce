package cartsService

import (
	cartsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/carts"
	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
	usersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/users"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	ErrInvalidContext       = utils.ErrContextInvalid
	ErrUsersServiceRequired = Error("cartsService: users service is required")
	ErrProductsRepoRequired = Error("cartsService: products repository is required")
	ErrCartsRepoRequired    = Error("cartsService: carts repository is required")
	ErrQuantityInvalid      = Error("cartsService: invalid quantity")
	ErrQuantityTooLarge     = Error("cartsService: quantity exceeds the per item limit")
	ErrOutOfStock           = Error("cartsService: not enough stock")
	ErrCartFull             = Error("cartsService: cart has too many items")
	ErrCartItemNotFound     = Error("cartsService: item not in cart")
	ErrBatchEmpty           = Error("cartsService: items are required")
	ErrTooManyItems         = Error("cartsService: too many items")
	ErrVariantIDInvalid     = cartsRepository.ErrVariantIDInvalid
	ErrVariantNotFound      = productsRepository.ErrVariantNotFound
	ErrTokenInvalid         = usersService.ErrTokenInvalid
	ErrTokenExpired         = usersService.ErrTokenExpired
	ErrTokenWrongType       = usersService.ErrTokenWrongType
	ErrUserNotFound         = usersService.ErrUserNotFound
)

type Error string

func (e Error) Error() string {
	return string(e)
}
