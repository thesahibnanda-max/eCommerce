package frontendRouter

const (
	ErrHandlerRequired = Error("frontendRouter: handler is required")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
