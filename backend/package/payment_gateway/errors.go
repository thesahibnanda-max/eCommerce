package paymentGateway

import (
	paymentsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/payments"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	ErrCtxInvalid              = utils.ErrContextInvalid
	ErrPaymentNotFound         = paymentsRepository.ErrPaymentNotFound
	ErrParamsInvalid           = Error("paymentGateway: constructor params are invalid")
	ErrClosed                  = Error("paymentGateway: gateway is closed")
	ErrOrderIDInvalid          = Error("paymentGateway: order id must be 1 to 128 printable characters")
	ErrUserIDInvalid           = Error("paymentGateway: user id must be 1 to 128 printable characters")
	ErrAmountInvalid           = Error("paymentGateway: amount is outside what this provider accepts")
	ErrPurposeInvalid          = Error("paymentGateway: purpose must be at most 100 printable characters")
	ErrPaymentIDInvalid        = Error("paymentGateway: invalid payment id")
	ErrPaymentForbidden        = Error("paymentGateway: payment belongs to another user")
	ErrPaymentInProgress       = Error("paymentGateway: a payment for this order is still being set up, try again shortly")
	ErrProviderFailed          = Error("paymentGateway: payment provider call failed")
	ErrNotSupported            = Error("paymentGateway: operation not supported by this provider")
	ErrWebhookSignatureInvalid = Error("paymentGateway: webhook signature is invalid")
	ErrWebhookInvalid          = Error("paymentGateway: webhook payload is invalid")
	ErrRedirectInvalid         = Error("paymentGateway: redirect query is invalid")
	ErrStatusInvalid           = Error("paymentGateway: status must be SUCCEEDED or FAILED")
	ErrNoteInvalid             = Error("paymentGateway: note must be at most 300 printable characters")
	ErrPanicked                = Error("paymentGateway: operation panicked")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
