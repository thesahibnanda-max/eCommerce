package paymentGateway

import (
	"log/slog"
	"os"
	"time"

	instamojoClient "github.com/thesahibnanda-max/eCommerce/backend/package/clients/instamojo"
	selfUPIClient "github.com/thesahibnanda-max/eCommerce/backend/package/clients/self_upi"
	paymentsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/payments"
)

type ConstructorParams interface {
	provider() Provider
}

type InstamojoConstructorParams struct {
	Client          instamojoClient.Client
	Payments        paymentsRepository.Repository
	PrivateSalt     string
	QueuePath       []string
	QueuePermission os.FileMode
	PollInterval    time.Duration
	PollWindow      time.Duration
	LeaseTimeout    time.Duration
	Concurrency     int
	RedirectURL     string
	WebhookURL      string
	Logger          *slog.Logger
	OnFinalized     FinalizedFunc
}

type UPIConstructorParams struct {
	Client      selfUPIClient.Client
	Payments    paymentsRepository.Repository
	PayeeName   string
	Logger      *slog.Logger
	OnFinalized FinalizedFunc
}

func (InstamojoConstructorParams) provider() Provider {
	return ProviderInstamojo
}

func (UPIConstructorParams) provider() Provider {
	return ProviderSelfUPI
}

func NewGateway(params ConstructorParams) (Gateway, error) {
	switch p := params.(type) {
	case InstamojoConstructorParams:
		return newInstamojoGateway(p)
	case *InstamojoConstructorParams:
		if p == nil {
			return nil, ErrParamsInvalid
		}
		return newInstamojoGateway(*p)
	case UPIConstructorParams:
		return newSelfUPIGateway(p)
	case *UPIConstructorParams:
		if p == nil {
			return nil, ErrParamsInvalid
		}
		return newSelfUPIGateway(*p)
	default:
		return nil, ErrParamsInvalid
	}
}
