package selfUPIClient

const (
	ErrDataInvalid        = Error("selfUPIClient: self_upi.json is invalid")
	ErrUPIIDInvalid       = Error("selfUPIClient: upi id is invalid")
	ErrQRImageInvalid     = Error("selfUPIClient: qr image is not a valid base64 image")
	ErrQRImageTooLarge    = Error("selfUPIClient: qr image is too large")
	ErrInitializePanicked = Error("selfUPIClient: initialization panicked")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
