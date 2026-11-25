package staticInfo

import (
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	ErrInvalidContext       = utils.ErrContextInvalid
	ErrCompanyNameRequired  = Error("staticInfo: company name is required")
	ErrSupportEmailInvalid  = Error("staticInfo: support email is invalid")
	ErrSupportPhoneInvalid  = Error("staticInfo: support phone is invalid")
	ErrAddressRequired      = Error("staticInfo: address is required")
	ErrGrievanceRequired    = Error("staticInfo: grievance officer is required")
	ErrEffectiveDateInvalid = Error("staticInfo: effective date must be YYYY-MM-DD")
	ErrReturnWindowInvalid  = Error("staticInfo: return window days must be positive")
	ErrTermsRequired        = Error("staticInfo: refund, dispatch and delivery days are required")
	ErrContentInvalid       = Error("staticInfo: embedded content is invalid")
	ErrSlugInvalid          = Error("staticInfo: invalid page slug")
	ErrPageNotFound         = Error("staticInfo: page not found")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
