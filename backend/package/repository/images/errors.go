package imagesRepository

import (
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	ErrCtxInvalid            = utils.ErrContextInvalid
	ErrIDInvalid             = Error("imagesRepository: invalid id")
	ErrImageNotFound         = Error("imagesRepository: image not found")
	ErrImageEmpty            = Error("imagesRepository: image content is empty")
	ErrImageTooLarge         = Error("imagesRepository: image content is too large")
	ErrImageTooManyPixels    = Error("imagesRepository: image has too many pixels")
	ErrBatchEmpty            = Error("imagesRepository: batch is empty")
	ErrImageDuplicateInBatch = Error("imagesRepository: image repeated within batch")
	ErrImageUnsupported      = Error("imagesRepository: unsupported or corrupt image")
	ErrLimitInvalid          = Error("imagesRepository: limit must be between 1 and 100")
	ErrOffsetInvalid         = Error("imagesRepository: offset must not be negative")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
