package rateLimiter

const (
	ErrConfigInvalid = Error("rateLimiter: invalid config")
	ErrGroupUnknown  = Error("rateLimiter: unknown group")
	ErrKeyRequired   = Error("rateLimiter: key is required")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
