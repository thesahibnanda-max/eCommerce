package utils

const (
	ErrPasswordTooShort          = Error("password must be at least 8 characters long")
	ErrPasswordTooLong           = Error("password must be at most 64 characters long")
	ErrPasswordInvalidCharacters = Error("password contains invalid characters")
	ErrPasswordTooWeak           = Error("password must contain at least one letter, one number, and one special character")
)

const ErrRandomNumberLength = Error("length must be between 1 and 128")

const ErrContextInvalid = Error("invalid context")

const ErrEnvVarMissing = Error("env var missing")

const ErrTimeLocationUnavailable = Error("time location is unavailable")

type Error string

func (e Error) Error() string {
	return string(e)
}
