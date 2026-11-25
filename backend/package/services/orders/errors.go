package ordersService

import (
	paymentGateway "github.com/thesahibnanda-max/eCommerce/backend/package/payment_gateway"
	addressRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/address"
	ordersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/orders"
	paymentsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/payments"
	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
	usersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/users"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	ErrInvalidContext         = utils.ErrContextInvalid
	ErrDependencyMissing      = Error("ordersService: a required dependency is missing")
	ErrPolicyInvalid          = Error("ordersService: shipping policy is invalid")
	ErrCartEmpty              = Error("ordersService: your bag is empty")
	ErrItemUnavailable        = Error("ordersService: an item in your bag is no longer available")
	ErrCurrencyUnsupported    = Error("ordersService: only INR prices can be checked out")
	ErrOutOfStock             = Error("ordersService: not enough stock for an item in your bag")
	ErrAddressRequired        = Error("ordersService: choose a delivery address")
	ErrOrderCannotPay         = Error("ordersService: this order can't be paid right now")
	ErrOrderCannotCancel      = Error("ordersService: this order can't be cancelled anymore")
	ErrProofNotAllowed        = Error("ordersService: payment proof can't be submitted for this order")
	ErrPaymentDecisionInvalid = Error("ordersService: decision must be SUCCEEDED or FAILED")
	ErrTrackingInvalid        = Error("ordersService: tracking needs a courier and number of at most 60 characters and an http(s) link of at most 512 characters")
	ErrNotSupported           = paymentGateway.ErrNotSupported
	ErrOrderNotFound          = ordersRepository.ErrOrderNotFound
	ErrOrderIDInvalid         = ordersRepository.ErrOrderIDInvalid
	ErrOrderStateInvalid      = ordersRepository.ErrStateInvalid
	ErrTransitionNotAllowed   = ordersRepository.ErrTransitionNotAllowed
	ErrIdempotencyKeyInvalid  = ordersRepository.ErrIdempotencyKeyInvalid
	ErrPaymentNotFound        = paymentsRepository.ErrPaymentNotFound
	ErrAddressNotFound        = addressRepository.ErrAddressNotFound
	ErrInsufficientStock      = productsRepository.ErrInsufficientStock
	ErrTokenInvalid           = usersService.ErrTokenInvalid
)

type Error string

func (e Error) Error() string {
	return string(e)
}
