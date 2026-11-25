package usersRepository

import (
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	ErrCtxInvalid               = utils.ErrContextInvalid
	ErrIDInvalid                = Error("usersRepository: invalid id")
	ErrNameRequired             = Error("usersRepository: name is required")
	ErrEmailRequired            = Error("usersRepository: email is required")
	ErrPhoneNumberRequired      = Error("usersRepository: phone number is required")
	ErrEmailInvalid             = Error("usersRepository: invalid email")
	ErrPhoneNumberInvalid       = Error("usersRepository: invalid phone number")
	ErrEmailAlreadyExists       = Error("usersRepository: email already exists")
	ErrPhoneNumberAlreadyExists = Error("usersRepository: phone number already exists")
	ErrUserNotFound             = Error("usersRepository: user not found")
	ErrIndexCorrupted           = Error("usersRepository: index points to missing user")
	ErrPasswordInvalid          = Error("usersRepository: invalid password")
	ErrInvalidCredentials       = Error("usersRepository: invalid credentials")
	ErrLimitInvalid             = Error("usersRepository: limit must be between 1 and 100")
	ErrOffsetInvalid            = Error("usersRepository: offset must not be negative")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
