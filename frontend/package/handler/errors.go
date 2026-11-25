package frontendHandler

const (
	ErrFilesRequired         = Error("frontendHandler: static files are required")
	ErrRateLimiterRequired   = Error("frontendHandler: rate limiter is required")
	ErrRateLimitGroupMissing = Error("frontendHandler: rate limit group is missing")
	ErrAPIPortInvalid        = Error("frontendHandler: api port must be between 1 and 65535")
	ErrIndexMissing          = Error("frontendHandler: static files have no index.html, run go generate ./frontend")
	ErrAssetsLoadFailed      = Error("frontendHandler: failed to load static files")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
