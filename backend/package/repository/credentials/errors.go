package credentialsRepository

import (
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	ErrCtxInvalid      = utils.ErrContextInvalid
	ErrKeyRequired     = Error("credentialsRepository: encryption key is required")
	ErrNameInvalid     = Error("credentialsRepository: credential name is required")
	ErrSecretRequired  = Error("credentialsRepository: credential secret is required")
	ErrNotFound        = Error("credentialsRepository: credential not found")
	ErrDecryptFailed   = Error("credentialsRepository: credential could not be decrypted, the encryption key may have changed")
	ErrRecordCorrupted = Error("credentialsRepository: stored credential is corrupted")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
