package productsRepository

import (
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	ErrCtxInvalid              = utils.ErrContextInvalid
	ErrIDInvalid               = Error("productsRepository: invalid id")
	ErrProductNotFound         = Error("productsRepository: product not found")
	ErrVariantNotFound         = Error("productsRepository: variant not found")
	ErrNameRequired            = Error("productsRepository: name is required")
	ErrVariantRequired         = Error("productsRepository: at least one variant is required")
	ErrSKURequired             = Error("productsRepository: sku is required")
	ErrSKUDuplicate            = Error("productsRepository: sku repeated within product")
	ErrSKUAlreadyExists        = Error("productsRepository: sku already exists")
	ErrPriceInvalid            = Error("productsRepository: invalid price")
	ErrCurrencyInvalid         = Error("productsRepository: invalid currency")
	ErrLimitInvalid            = Error("productsRepository: invalid limit")
	ErrOffsetInvalid           = Error("productsRepository: invalid offset")
	ErrSKUDuplicateInBatch     = Error("productsRepository: sku repeated within batch")
	ErrProductDuplicateInBatch = Error("productsRepository: product repeated within batch")
	ErrBatchEmpty              = Error("productsRepository: batch is empty")
	ErrStockChangeInvalid      = Error("productsRepository: stock change needs a variant id and a non-zero quantity")
	ErrInsufficientStock       = Error("productsRepository: not enough stock")
	ErrSearchIndex             = Error("productsRepository: product saved but search index update failed")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
