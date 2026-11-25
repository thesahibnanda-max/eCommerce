package paymentGateway

import (
	paymentsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/payments"
)

type (
	Provider   = paymentsRepository.Provider
	State      = paymentsRepository.State
	ActionType string
)

const (
	ProviderInstamojo = paymentsRepository.ProviderInstamojo
	ProviderSelfUPI   = paymentsRepository.ProviderSelfUPI

	StateCreated         = paymentsRepository.StateCreated
	StateAwaitingPayment = paymentsRepository.StateAwaitingPayment
	StateVerifying       = paymentsRepository.StateVerifying
	StateSucceeded       = paymentsRepository.StateSucceeded
	StateFailed          = paymentsRepository.StateFailed
	StateExpired         = paymentsRepository.StateExpired

	ActionRedirect ActionType = "REDIRECT"
	ActionShowQR   ActionType = "SHOW_QR"
)

func (a ActionType) String() string {
	return string(a)
}

func (a ActionType) IsValid() bool {
	return a == ActionRedirect || a == ActionShowQR
}
