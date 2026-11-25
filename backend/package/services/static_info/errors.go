package staticInfoService

import (
	staticInfo "github.com/thesahibnanda-max/eCommerce/backend/package/static_info"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	ErrInvalidContext     = utils.ErrContextInvalid
	ErrStaticInfoRequired = Error("staticInfoService: static info is required")
	ErrSlugInvalid        = staticInfo.ErrSlugInvalid
	ErrPageNotFound       = staticInfo.ErrPageNotFound
)

type Error string

func (e Error) Error() string {
	return string(e)
}
