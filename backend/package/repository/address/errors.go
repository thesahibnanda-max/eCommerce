package addressRepository

import (
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	ErrCtxInvalid           = utils.ErrContextInvalid
	ErrUserIDInvalid        = Error("addressRepository: invalid user id")
	ErrAddressIDInvalid     = Error("addressRepository: invalid address id")
	ErrAddressNotFound      = Error("addressRepository: address not found")
	ErrNoDefaultAddress     = Error("addressRepository: no default address")
	ErrAddressLimitReached  = Error("addressRepository: address limit reached")
	ErrFullNameRequired     = Error("addressRepository: full name is required")
	ErrFullNameTooLong      = Error("addressRepository: full name is too long")
	ErrPhoneNumberRequired  = Error("addressRepository: phone number is required")
	ErrPhoneNumberInvalid   = Error("addressRepository: invalid phone number")
	ErrAddressLine1Required = Error("addressRepository: address line 1 is required")
	ErrAddressLine1TooLong  = Error("addressRepository: address line 1 is too long")
	ErrAddressLine2TooLong  = Error("addressRepository: address line 2 is too long")
	ErrLandmarkTooLong      = Error("addressRepository: landmark is too long")
	ErrCityTooLong          = Error("addressRepository: city is too long")
	ErrLabelTooLong         = Error("addressRepository: label is too long")
	ErrTextInvalid          = Error("addressRepository: text contains invalid characters")
	ErrPincodeRequired      = Error("addressRepository: pincode is required")
	ErrPincodeInvalid       = Error("addressRepository: invalid pincode")
	ErrPincodeNotFound      = Error("addressRepository: pincode not serviceable")
	ErrPincodeLookupFailed  = Error("addressRepository: pincode lookup failed")
	ErrCountryUnsupported   = Error("addressRepository: only addresses in India are supported")
	ErrMetadataTooLarge     = Error("addressRepository: metadata is too large")
	ErrMutateRequired       = Error("addressRepository: mutate function is required")
	ErrMutatePanicked       = Error("addressRepository: mutate function panicked")
	ErrIndexCorrupted       = Error("addressRepository: address index is corrupted")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
