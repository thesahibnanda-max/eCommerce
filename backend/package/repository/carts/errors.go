package cartsRepository

import (
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	ErrCtxInvalid       = utils.ErrContextInvalid
	ErrUserIDInvalid    = Error("cartsRepository: invalid user id")
	ErrVariantIDInvalid = Error("cartsRepository: invalid variant id")
	ErrQuantityInvalid  = Error("cartsRepository: quantity must be at least 1")
	ErrItemDuplicate    = Error("cartsRepository: duplicate variant in cart")
	ErrMutateRequired   = Error("cartsRepository: mutate function is required")
	ErrMutatePanicked   = Error("cartsRepository: mutate function panicked")
	ErrLimitInvalid     = Error("cartsRepository: limit must be between 1 and 100")
	ErrOffsetInvalid    = Error("cartsRepository: offset must not be negative")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
