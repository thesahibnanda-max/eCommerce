package paymentGateway

import (
	"context"
)

type Gateway interface {
	Provider() Provider
	InitiatePayment(ctx context.Context, req InitiateRequest) (InitiateResponse, error)
	SubmitProof(ctx context.Context, req SubmitProofRequest) (Payment, error)
	HandleWebhook(ctx context.Context, req WebhookRequest) (Payment, error)
	HandleRedirect(ctx context.Context, req RedirectRequest) (Payment, error)
	GetPayment(ctx context.Context, paymentID string) (Payment, error)
	Close() error
}

var (
	_ Gateway = (*instamojoGateway)(nil)
	_ Gateway = (*selfUPIGateway)(nil)
)
