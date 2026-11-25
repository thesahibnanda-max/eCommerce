package dependencyInjection

const (
	ErrConfigInvalid            = Error("dependencyInjection: invalid config")
	ErrServerPortInvalid        = Error("dependencyInjection: server port must be between 1 and 65535")
	ErrStorageDirectoryRequired = Error("dependencyInjection: storage directory is required")
	ErrStorageDirectoryFailed   = Error("dependencyInjection: failed to create storage directory")
	ErrStoragePermissionInvalid = Error("dependencyInjection: storage permission must be an octal file mode")
	ErrSMTPSecurityInvalid      = Error("dependencyInjection: smtp security must be starttls, ssl or none")
	ErrPaymentProviderInvalid   = Error("dependencyInjection: payments provider must be SELF_UPI or INSTAMOJO")
	ErrCloseFailed              = Error("dependencyInjection: failed to close resources")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
