package paymentGateway

import (
	"context"
	"net/url"
	"strings"

	selfUPIClient "github.com/thesahibnanda-max/eCommerce/backend/package/clients/self_upi"
	paymentsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/payments"
)

const (
	upiMinAmount       = 1
	upiMaxAmount       = 100000
	upiMaxPayeeRunes   = 99
	upiMaxNoteRunes    = 50
	upiStatusSucceeded = "SUCCEEDED"
	upiStatusFailed    = "FAILED"
)

type selfUPIGateway struct {
	*core
	client    selfUPIClient.Client
	payeeName string
}

func newSelfUPIGateway(p UPIConstructorParams) (*selfUPIGateway, error) {
	payeeName, ok := cleanText(strings.Join(strings.Fields(p.PayeeName), " "), upiMaxPayeeRunes)
	if p.Client == nil || p.Payments == nil || !ok || payeeName == "" {
		return nil, ErrParamsInvalid
	}
	return &selfUPIGateway{
		core:      newCore(ProviderSelfUPI, p.Payments, p.Logger, p.OnFinalized),
		client:    p.Client,
		payeeName: payeeName,
	}, nil
}

func (g *selfUPIGateway) InitiatePayment(ctx context.Context, req InitiateRequest) (resp InitiateResponse, err error) {
	defer recoverAsError(&err)
	if err := g.ready(ctx); err != nil {
		return InitiateResponse{}, err
	}
	model, _, err := g.create(ctx, req, upiMinAmount, upiMaxAmount, map[string]any{"upi_id": g.client.GetUPIID()})
	if err != nil {
		return InitiateResponse{}, err
	}
	if model.State == StateCreated {
		model, _, err = g.transition(ctx, model.ID, StateAwaitingPayment, paymentsRepository.Patch{}, paymentsRepository.EventInput{
			Source: paymentsRepository.EventSourceSystem,
			Kind:   "awaiting_payment",
		})
		if err != nil {
			return InitiateResponse{}, err
		}
	}
	return InitiateResponse{
		Payment: toPayment(model),
		Action: NextAction{
			Type:           ActionShowQR,
			QRImageDataURI: g.client.GetUPIQRDataURI(),
			UPIID:          g.client.GetUPIID(),
			UPIIntentURL:   g.intentURL(model),
		},
	}, nil
}

func (g *selfUPIGateway) HandleWebhook(ctx context.Context, req WebhookRequest) (payment Payment, err error) {
	defer recoverAsError(&err)
	if err := g.ready(ctx); err != nil {
		return Payment{}, err
	}
	fields, err := singleValues(req.Form)
	if err != nil {
		return Payment{}, err
	}
	var to State
	switch strings.ToUpper(fields["status"]) {
	case upiStatusSucceeded:
		to = StateSucceeded
	case upiStatusFailed:
		to = StateFailed
	default:
		return Payment{}, ErrStatusInvalid
	}
	note, ok := cleanText(fields["note"], maxNoteRunes)
	if !ok {
		return Payment{}, ErrNoteInvalid
	}
	model, err := g.load(ctx, fields["payment_id"])
	if err != nil {
		return Payment{}, err
	}
	patch := paymentsRepository.Patch{}
	if to == StateFailed {
		reason := note
		if reason == "" {
			reason = "payment rejected by admin"
		}
		patch.FailureReason = &reason
	} else {
		cleared := ""
		patch.FailureReason = &cleared
	}
	model, _, err = g.transition(ctx, model.ID, to, patch, paymentsRepository.EventInput{
		Source: paymentsRepository.EventSourceAdmin,
		Kind:   "admin_marked_" + strings.ToLower(to.String()),
		Data:   map[string]any{"note": note},
	})
	if err != nil {
		return Payment{}, err
	}
	return toPayment(model), nil
}

func (g *selfUPIGateway) HandleRedirect(ctx context.Context, _ RedirectRequest) (Payment, error) {
	if err := g.ready(ctx); err != nil {
		return Payment{}, err
	}
	return Payment{}, ErrNotSupported
}

func (g *selfUPIGateway) Close() error {
	g.closed.Store(true)
	return nil
}

func (g *selfUPIGateway) intentURL(model paymentsRepository.Model) string {
	note := truncate("Order "+model.OrderID, upiMaxNoteRunes)
	params := [][2]string{
		{"pa", g.client.GetUPIID()},
		{"pn", g.payeeName},
		{"am", formatAmount(model.Amount)},
		{"cu", paymentsRepository.CurrencyINR.String()},
		{"tr", model.ID},
		{"tn", note},
	}
	parts := make([]string, 0, len(params))
	for _, p := range params {
		parts = append(parts, p[0]+"="+url.PathEscape(p[1]))
	}
	return "upi://pay?" + strings.Join(parts, "&")
}
