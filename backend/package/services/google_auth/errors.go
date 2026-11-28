package googleAuthService

import "github.com/thesahibnanda-max/eCommerce/backend/package/utils"

const (
	ErrCtxInvalid     = utils.ErrContextInvalid
	ErrParamsInvalid  = Error("googleAuthService: client id, client secret, a positive timeout and a credentials repository are required")
	ErrURLInvalid     = Error("googleAuthService: callback and return urls must be absolute http or https urls")
	ErrStateInvalid   = Error("googleAuthService: the google sign-in link expired or was already used")
	ErrConsentDenied  = Error("googleAuthService: google access was not granted")
	ErrExchangeFailed = Error("googleAuthService: google sign-in could not be completed")
	ErrNoRefreshToken = Error("googleAuthService: google did not return a refresh token")
	ErrNotConnected   = Error("googleAuthService: google drive is not connected")
	ErrClosed         = Error("googleAuthService: service is closed")
	ErrPanicked       = Error("googleAuthService: recovered from panic")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
