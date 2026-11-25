package smtp

const (
	ErrNoSenders         = Error("smtp: at least one sender is required")
	ErrSenderInvalid     = Error("smtp: invalid sender")
	ErrRecipientRequired = Error("smtp: recipient is required")
	ErrRecipientInvalid  = Error("smtp: invalid recipient")
	ErrSubjectRequired   = Error("smtp: subject is required")
	ErrSubjectInvalid    = Error("smtp: subject must be a single line")
	ErrBodyRequired      = Error("smtp: body or html body is required")
	ErrSendFailed        = Error("smtp: send failed")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
