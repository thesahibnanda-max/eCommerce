package router

const (
	ErrHandlerRequired     = Error("router: handler is required")
	ErrFrontendPortInvalid = Error("router: frontend port must be between 1 and 65535")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
