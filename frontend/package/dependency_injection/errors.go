package frontendDependencyInjection

const (
	ErrConfigInvalid     = Error("frontendDependencyInjection: invalid config")
	ErrServerPortInvalid = Error("frontendDependencyInjection: server port must be between 1 and 65535")
	ErrStaticFilesFailed = Error("frontendDependencyInjection: failed to open static files")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
