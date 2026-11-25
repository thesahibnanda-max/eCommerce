package otp

const (
	ErrMailerRequired   = Error("otp: mailer is required")
	ErrConfigInvalid    = Error("otp: invalid config")
	ErrRecipientInvalid = Error("otp: invalid recipient")
	ErrResendTooSoon    = Error("otp: please wait before requesting another otp")
	ErrSendFailed       = Error("otp: failed to send otp")
	ErrOTPNotFound      = Error("otp: no otp found for recipient")
	ErrOTPExpired       = Error("otp: otp has expired")
	ErrOTPInvalid       = Error("otp: invalid otp")
	ErrTooManyAttempts  = Error("otp: too many invalid attempts, request a new otp")
	ErrSMSNotSupported  = Error("otp: sms otp not supported yet")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
