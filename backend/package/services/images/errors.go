package imagesService

import (
	imagesRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/images"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	ErrInvalidContext        = utils.ErrContextInvalid
	ErrImagesRepoRequired    = Error("imagesService: images repository is required")
	ErrBatchEmpty            = Error("imagesService: ids are required")
	ErrTooManyIDs            = Error("imagesService: too many ids")
	ErrImageDuplicateInBatch = Error("imagesService: image repeated within batch")
	ErrImageNotFound         = imagesRepository.ErrImageNotFound
	ErrIDInvalid             = imagesRepository.ErrIDInvalid
)

type Error string

func (e Error) Error() string {
	return string(e)
}
