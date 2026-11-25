package ordersRepository

import (
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	ErrCtxInvalid             = utils.ErrContextInvalid
	ErrOrderNotFound          = Error("ordersRepository: order not found")
	ErrOrderIDInvalid         = Error("ordersRepository: invalid order id")
	ErrUserIDInvalid          = Error("ordersRepository: invalid user id")
	ErrIdempotencyKeyInvalid  = Error("ordersRepository: idempotency key must be 1 to 128 characters")
	ErrIdempotencyKeyConflict = Error("ordersRepository: idempotency key was used for a different order")
	ErrCurrencyInvalid        = Error("ordersRepository: unsupported currency")
	ErrItemsRequired          = Error("ordersRepository: order needs at least one item")
	ErrTooManyItems           = Error("ordersRepository: order has too many items")
	ErrItemInvalid            = Error("ordersRepository: item is missing product, variant, sku or name")
	ErrDuplicateVariant       = Error("ordersRepository: duplicate variant in order")
	ErrQuantityInvalid        = Error("ordersRepository: quantity must be at least 1")
	ErrPriceInvalid           = Error("ordersRepository: unit price must be at least 1")
	ErrAmountInvalid          = Error("ordersRepository: shipping fee and discount must be non-negative and the total cannot be negative")
	ErrAmountOverflow         = Error("ordersRepository: order amount overflows")
	ErrAddressIncomplete      = Error("ordersRepository: shipping address is incomplete")
	ErrStateInvalid           = Error("ordersRepository: invalid order state")
	ErrTransitionNotAllowed   = Error("ordersRepository: order state transition not allowed")
	ErrConcurrentUpdate       = Error("ordersRepository: order changed concurrently")
	ErrActorInvalid           = Error("ordersRepository: invalid actor")
	ErrReasonTooLong          = Error("ordersRepository: reason is too long")
	ErrPaymentGatewayInvalid  = Error("ordersRepository: payment gateway must be 1 to 64 characters")
	ErrPaymentGatewayLocked   = Error("ordersRepository: payment gateway can only change while the order is created or processing")
	ErrPaymentEventInvalid    = Error("ordersRepository: payment event needs gateway, event id and event type and a non-negative amount")
	ErrPaymentEventDuplicate  = Error("ordersRepository: payment event already recorded")
	ErrCursorInvalid          = Error("ordersRepository: invalid cursor")
	ErrLimitInvalid           = Error("ordersRepository: limit must be between 1 and 100")
	ErrMetadataTooLarge       = Error("ordersRepository: metadata is too large")
	ErrMutateRequired         = Error("ordersRepository: mutate function is required")
	ErrMutatePanicked         = Error("ordersRepository: mutate function panicked")
	ErrTransactionPanicked    = Error("ordersRepository: transaction panicked")
	ErrStoredDataCorrupted    = Error("ordersRepository: stored order data is corrupted")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
