package token

const (
	ErrSecretTooShort = Error("token: secret must be at least 32 bytes")
	ErrPayloadInvalid = Error("token: payload is invalid")
	ErrPayloadEncode  = Error("token: payload cannot be encoded")
	ErrTokenInvalid   = Error("token: token is invalid")
	ErrTokenExpired   = Error("token: token is expired")
	ErrTokenWrongType = Error("token: token has the wrong type")
	ErrTokenReused    = Error("token: refresh token was already used")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
