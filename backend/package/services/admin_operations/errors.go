package adminOperationsService

import (
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	ErrInvalidContext    = utils.ErrContextInvalid
	ErrAdminKeyRequired  = Error("adminOperationsService: admin key is required")
	ErrDependencyMissing = Error("adminOperationsService: a required dependency is missing")
	ErrUnauthorized      = Error("adminOperationsService: invalid admin key")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
