package handler

const (
	ErrUsersServiceRequired      = Error("handler: users service is required")
	ErrAdminServiceRequired      = Error("handler: admin service is required")
	ErrProductsServiceRequired   = Error("handler: products service is required")
	ErrImagesServiceRequired     = Error("handler: images service is required")
	ErrCartsServiceRequired      = Error("handler: carts service is required")
	ErrStaticInfoServiceRequired = Error("handler: static info service is required")
	ErrAddressesServiceRequired  = Error("handler: addresses service is required")
	ErrOrdersServiceRequired     = Error("handler: orders service is required")
	ErrAdminOpsServiceRequired   = Error("handler: admin operations service is required")
	ErrRateLimiterRequired       = Error("handler: rate limiter is required")
	ErrRateLimitGroupMissing     = Error("handler: rate limit group is missing")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
