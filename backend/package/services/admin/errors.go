package adminService

import (
	imagesRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/images"
	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	ErrInvalidContext          = utils.ErrContextInvalid
	ErrAdminKeyRequired        = Error("adminService: admin key is required")
	ErrProductsRepoRequired    = Error("adminService: products repository is required")
	ErrImagesRepoRequired      = Error("adminService: images repository is required")
	ErrUnauthorized            = Error("adminService: unauthorized")
	ErrBatchEmpty              = Error("adminService: batch is empty")
	ErrBatchTooLarge           = Error("adminService: batch is too large")
	ErrProductNotFound         = productsRepository.ErrProductNotFound
	ErrVariantNotFound         = productsRepository.ErrVariantNotFound
	ErrProductIDInvalid        = productsRepository.ErrIDInvalid
	ErrNameRequired            = productsRepository.ErrNameRequired
	ErrVariantRequired         = productsRepository.ErrVariantRequired
	ErrSKURequired             = productsRepository.ErrSKURequired
	ErrSKUDuplicate            = productsRepository.ErrSKUDuplicate
	ErrSKUDuplicateInBatch     = productsRepository.ErrSKUDuplicateInBatch
	ErrSKUAlreadyExists        = productsRepository.ErrSKUAlreadyExists
	ErrProductDuplicateInBatch = productsRepository.ErrProductDuplicateInBatch
	ErrPriceInvalid            = productsRepository.ErrPriceInvalid
	ErrCurrencyInvalid         = productsRepository.ErrCurrencyInvalid
	ErrSearchIndex             = productsRepository.ErrSearchIndex
	ErrImageIDInvalid          = imagesRepository.ErrIDInvalid
	ErrImageNotFound           = imagesRepository.ErrImageNotFound
	ErrImageEmpty              = imagesRepository.ErrImageEmpty
	ErrImageTooLarge           = imagesRepository.ErrImageTooLarge
	ErrImageTooManyPixels      = imagesRepository.ErrImageTooManyPixels
	ErrImageUnsupported        = imagesRepository.ErrImageUnsupported
	ErrImageDuplicateInBatch   = imagesRepository.ErrImageDuplicateInBatch
)

type Error string

func (e Error) Error() string {
	return string(e)
}
