package instamojoClient

import (
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	ErrCtxInvalid              = utils.ErrContextInvalid
	ErrConfigInvalid           = Error("instamojoClient: base url, client id, client secret and timeout are required")
	ErrAmountInvalid           = Error("instamojoClient: amount must be between 9 and 200000 rupees")
	ErrPurposeInvalid          = Error("instamojoClient: purpose must be 1 to 30 characters")
	ErrBuyerNameInvalid        = Error("instamojoClient: buyer name must be at most 100 characters")
	ErrEmailInvalid            = Error("instamojoClient: email is invalid")
	ErrEmailRequired           = Error("instamojoClient: email is required when send email is true")
	ErrPhoneInvalid            = Error("instamojoClient: phone is invalid")
	ErrRedirectURLInvalid      = Error("instamojoClient: redirect url must be an absolute http or https url")
	ErrWebhookURLInvalid       = Error("instamojoClient: webhook url must be an absolute http or https url")
	ErrExpiresAtInvalid        = Error("instamojoClient: expires at must be in the future")
	ErrPaymentRequestIDInvalid = Error("instamojoClient: payment request id must be 32 lowercase hex characters")
	ErrPaymentIDInvalid        = Error("instamojoClient: payment id is invalid")
	ErrNotFound                = Error("instamojoClient: not found on instamojo")
	ErrAuthFailed              = Error("instamojoClient: could not get access token")
	ErrUnauthorized            = Error("instamojoClient: access token rejected")
	ErrForbidden               = Error("instamojoClient: request not permitted for this account")
	ErrValidationFailed        = Error("instamojoClient: instamojo rejected the request")
	ErrUpstream                = Error("instamojoClient: instamojo request failed")
	ErrResponseInvalid         = Error("instamojoClient: instamojo response is invalid")
	ErrClosed                  = Error("instamojoClient: client is closed")
	ErrRequestPanicked         = Error("instamojoClient: request panicked")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
