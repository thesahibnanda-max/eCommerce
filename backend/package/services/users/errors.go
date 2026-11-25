package usersService

import (
	"github.com/thesahibnanda-max/eCommerce/backend/package/otp"
	usersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/users"
	"github.com/thesahibnanda-max/eCommerce/backend/package/token"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	ErrInvalidContext        = utils.ErrContextInvalid
	ErrInvalidCredentials    = usersRepository.ErrInvalidCredentials
	ErrUserNotFound          = usersRepository.ErrUserNotFound
	ErrTokenInvalid          = token.ErrTokenInvalid
	ErrTokenExpired          = token.ErrTokenExpired
	ErrTokenWrongType        = token.ErrTokenWrongType
	ErrTokenReused           = token.ErrTokenReused
	ErrTokenizerRequired     = Error("usersService: tokenizer is required")
	ErrUsersRepoRequired     = Error("usersService: users repository is required")
	ErrContactRequired       = Error("usersService: email or phone number is required")
	ErrEmailOTPRequired      = Error("usersService: email otp is required")
	ErrEmailAlreadyValidated = Error("usersService: email is already validated")
	ErrEmailRequired         = usersRepository.ErrEmailRequired
	ErrPhoneNumberRequired   = usersRepository.ErrPhoneNumberRequired
	ErrUserIDInvalid         = usersRepository.ErrIDInvalid
	ErrNameRequired          = usersRepository.ErrNameRequired
	ErrEmailInvalid          = usersRepository.ErrEmailInvalid
	ErrPhoneNumberInvalid    = usersRepository.ErrPhoneNumberInvalid
	ErrEmailAlreadyExists    = usersRepository.ErrEmailAlreadyExists
	ErrPhoneAlreadyExists    = usersRepository.ErrPhoneNumberAlreadyExists
	ErrPasswordInvalid       = usersRepository.ErrPasswordInvalid
	ErrOTPRecipientInvalid   = otp.ErrRecipientInvalid
	ErrOTPSendFailed         = otp.ErrSendFailed
	ErrOTPInvalid            = otp.ErrOTPInvalid
	ErrOTPExpired            = otp.ErrOTPExpired
	ErrOTPNotFound           = otp.ErrOTPNotFound
	ErrOTPTooManyAttempts    = otp.ErrTooManyAttempts
	ErrOTPResendTooSoon      = otp.ErrResendTooSoon
)

type Error string

func (e Error) Error() string {
	return string(e)
}
