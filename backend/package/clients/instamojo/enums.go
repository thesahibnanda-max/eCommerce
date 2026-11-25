package instamojoClient

type PaymentRequestStatus string

const (
	PaymentRequestStatusPending   PaymentRequestStatus = "Pending"
	PaymentRequestStatusSent      PaymentRequestStatus = "Sent"
	PaymentRequestStatusCompleted PaymentRequestStatus = "Completed"
	PaymentRequestStatusFailed    PaymentRequestStatus = "Failed"
	PaymentRequestStatusExpired   PaymentRequestStatus = "Expired"
)

func (s PaymentRequestStatus) String() string {
	return string(s)
}

func (s PaymentRequestStatus) IsValid() bool {
	switch s {
	case PaymentRequestStatusPending, PaymentRequestStatusSent, PaymentRequestStatusCompleted, PaymentRequestStatusFailed, PaymentRequestStatusExpired:
		return true
	default:
		return false
	}
}

func (s PaymentRequestStatus) IsTerminal() bool {
	switch s {
	case PaymentRequestStatusCompleted, PaymentRequestStatusFailed, PaymentRequestStatusExpired:
		return true
	default:
		return false
	}
}

type PaymentStatus string

const (
	PaymentStatusInitiated  PaymentStatus = "INITIATED"
	PaymentStatusSuccessful PaymentStatus = "SUCCESSFUL"
	PaymentStatusFailed     PaymentStatus = "FAILED"
)

func (s PaymentStatus) String() string {
	return string(s)
}

func (s PaymentStatus) IsValid() bool {
	switch s {
	case PaymentStatusInitiated, PaymentStatusSuccessful, PaymentStatusFailed:
		return true
	default:
		return false
	}
}

func (s PaymentStatus) IsTerminal() bool {
	return s == PaymentStatusSuccessful || s == PaymentStatusFailed
}

func (p Payment) PaymentStatus() PaymentStatus {
	switch {
	case p.Status == nil:
		return PaymentStatusInitiated
	case *p.Status:
		return PaymentStatusSuccessful
	default:
		return PaymentStatusFailed
	}
}

type InstrumentType string

const (
	InstrumentTypeCard       InstrumentType = "CARD"
	InstrumentTypeEMI        InstrumentType = "EMI"
	InstrumentTypeNetbanking InstrumentType = "NETBANKING"
	InstrumentTypeUPI        InstrumentType = "UPI"
	InstrumentTypeWallet     InstrumentType = "WALLET"
	InstrumentTypeQR         InstrumentType = "QR"
)

func (t InstrumentType) String() string {
	return string(t)
}

func (t InstrumentType) IsKnown() bool {
	switch t {
	case InstrumentTypeCard, InstrumentTypeEMI, InstrumentTypeNetbanking, InstrumentTypeUPI, InstrumentTypeWallet, InstrumentTypeQR:
		return true
	default:
		return false
	}
}
