package paymentsRepository

import (
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	ErrCtxInvalid             = utils.ErrContextInvalid
	ErrPaymentNotFound        = Error("paymentsRepository: payment not found")
	ErrPaymentIDInvalid       = Error("paymentsRepository: invalid payment id")
	ErrOrderIDInvalid         = Error("paymentsRepository: order id must be 1 to 128 printable characters")
	ErrUserIDInvalid          = Error("paymentsRepository: user id must be 1 to 128 printable characters")
	ErrProviderInvalid        = Error("paymentsRepository: invalid provider")
	ErrAmountInvalid          = Error("paymentsRepository: amount must be at least 1")
	ErrPurposeInvalid         = Error("paymentsRepository: purpose must be at most 100 printable characters")
	ErrStateInvalid           = Error("paymentsRepository: invalid payment state")
	ErrTransitionNotAllowed   = Error("paymentsRepository: payment state transition not allowed")
	ErrConcurrentUpdate       = Error("paymentsRepository: payment changed concurrently")
	ErrOpenPaymentConflict    = Error("paymentsRepository: order already has an open payment with a different provider or amount")
	ErrReferenceInvalid       = Error("paymentsRepository: gateway reference, payment id or checkout url is invalid")
	ErrReferenceTaken         = Error("paymentsRepository: gateway reference already belongs to another payment")
	ErrFailureReasonTooLong   = Error("paymentsRepository: failure reason is too long")
	ErrEventSourceInvalid     = Error("paymentsRepository: invalid event source")
	ErrEventKindInvalid       = Error("paymentsRepository: event kind must be 1 to 64 characters of letters, digits or '_'")
	ErrEventDedupeKeyInvalid  = Error("paymentsRepository: event dedupe key must be at most 256 printable characters")
	ErrEventDuplicate         = Error("paymentsRepository: event already recorded")
	ErrEventDataTooLarge      = Error("paymentsRepository: event data is too large")
	ErrMetadataTooLarge       = Error("paymentsRepository: metadata is too large")
	ErrProofNotAllowed        = Error("paymentsRepository: proof can only be submitted while awaiting payment or verifying")
	ErrProofTransactionID     = Error("paymentsRepository: transaction id must be 6 to 35 letters or digits")
	ErrProofTransactionReused = Error("paymentsRepository: transaction id was already used for another payment")
	ErrProofImageInvalid      = Error("paymentsRepository: proof image must be a jpeg, png or webp image")
	ErrProofImageTooLarge     = Error("paymentsRepository: proof image is too large")
	ErrProofNotFound          = Error("paymentsRepository: payment has no proof image")
	ErrBatchTooLarge          = Error("paymentsRepository: too many ids in one batch")
	ErrCursorInvalid          = Error("paymentsRepository: invalid cursor")
	ErrLimitInvalid           = Error("paymentsRepository: limit must be between 1 and 100")
	ErrTransactionPanicked    = Error("paymentsRepository: transaction panicked")
	ErrStoredDataCorrupted    = Error("paymentsRepository: stored payment data is corrupted")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
