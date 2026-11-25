package productsService

import (
	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	ErrInvalidContext       = utils.ErrContextInvalid
	ErrProductsRepoRequired = Error("productsService: products repository is required")
	ErrBatchEmpty           = Error("productsService: ids are required")
	ErrTooManyIDs           = Error("productsService: too many ids")
	ErrSortInvalid          = Error("productsService: invalid sort")
	ErrProductNotFound      = productsRepository.ErrProductNotFound
	ErrVariantNotFound      = productsRepository.ErrVariantNotFound
	ErrIDInvalid            = productsRepository.ErrIDInvalid
	ErrLimitInvalid         = productsRepository.ErrLimitInvalid
	ErrOffsetInvalid        = productsRepository.ErrOffsetInvalid
	ErrPriceInvalid         = productsRepository.ErrPriceInvalid
)

type Error string

func (e Error) Error() string {
	return string(e)
}
