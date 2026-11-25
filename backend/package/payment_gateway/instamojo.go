package paymentGateway

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	instamojoClient "github.com/thesahibnanda-max/eCommerce/backend/package/clients/instamojo"
	paymentsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/payments"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	instamojoPollJob   = "instamojo_poll"
	instamojoMinAmount = 9
	instamojoMaxAmount = 200000
	instamojoCurrency  = "INR"
)

type pollMessage struct {
	PaymentID string `json:"payment_id"`
	Deadline  int64  `json:"deadline"`
}

type instamojoGateway struct {
	*core
	client       instamojoClient.Client
	salt         string
	queue        *jobQueue
	pollInterval time.Duration
	pollWindow   time.Duration
	redirectURL  string
	webhookURL   string
}

func newInstamojoGateway(p InstamojoConstructorParams) (*instamojoGateway, error) {
	salt := strings.TrimSpace(p.PrivateSalt)
	redirectURL := strings.TrimSpace(p.RedirectURL)
	webhookURL := strings.TrimSpace(p.WebhookURL)
	switch {
	case p.Client == nil, p.Payments == nil, salt == "", len(p.QueuePath) == 0, p.QueuePermission == 0:
		return nil, ErrParamsInvalid
	case p.PollInterval <= 0, p.PollWindow < p.PollInterval, p.LeaseTimeout <= 0, p.Concurrency <= 0:
		return nil, ErrParamsInvalid
	case redirectURL != "" && !isWebURL(redirectURL), webhookURL != "" && !isWebURL(webhookURL):
		return nil, ErrParamsInvalid
	}
	queue, err := newJobQueue(queueConfig{
		path:         p.QueuePath,
		permission:   p.QueuePermission,
		leaseTimeout: p.LeaseTimeout,
		pollInterval: p.PollInterval,
		concurrency:  p.Concurrency,
		logger:       p.Logger,
	})
	if err != nil {
		return nil, err
	}
	g := &instamojoGateway{
		core:         newCore(ProviderInstamojo, p.Payments, p.Logger, p.OnFinalized),
		client:       p.Client,
		salt:         salt,
		queue:        queue,
		pollInterval: p.PollInterval,
		pollWindow:   p.PollWindow,
		redirectURL:  redirectURL,
		webhookURL:   webhookURL,
	}
	queue.register(instamojoPollJob, g.poll)
	queue.start()
	return g, nil
}

func (g *instamojoGateway) InitiatePayment(ctx context.Context, req InitiateRequest) (resp InitiateResponse, err error) {
	defer recoverAsError(&err)
	if err := g.ready(ctx); err != nil {
		return InitiateResponse{}, err
	}
	deadline := g.now().Add(g.pollWindow)
	model, created, err := g.create(ctx, req, instamojoMinAmount, instamojoMaxAmount, map[string]any{
		metadataDeadlineKey: deadline.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return InitiateResponse{}, err
	}
	if !created {
		if model.CheckoutURL == "" {
			return InitiateResponse{}, ErrPaymentInProgress
		}
		return redirectResponse(model), nil
	}
	if err := g.queue.enqueue(ctx, instamojoPollJob, pollMessage{PaymentID: model.ID, Deadline: deadline.UnixMicro()}, g.pollInterval); err != nil {
		g.markFailed(ctx, model.ID, "could not schedule payment verification")
		return InitiateResponse{}, err
	}
	request, err := g.client.CreatePaymentRequest(ctx, instamojoClient.CreatePaymentRequestRequest{
		Amount:      model.Amount,
		Purpose:     model.ID,
		BuyerName:   req.BuyerName,
		Email:       req.Email,
		Phone:       req.Phone,
		RedirectURL: g.redirectURL,
		WebhookURL:  g.webhookURL,
		ExpiresAt:   deadline,
	})
	if err != nil {
		g.markFailedWithDetail(ctx, model.ID, reasonPaymentSetupFailed, err.Error())
		return InitiateResponse{}, fmt.Errorf("%w: %w", ErrProviderFailed, err)
	}
	model, _, err = g.transition(ctx, model.ID, StateAwaitingPayment, paymentsRepository.Patch{
		GatewayReference: &request.ID,
		CheckoutURL:      &request.LongURL,
	}, paymentsRepository.EventInput{
		Source:    paymentsRepository.EventSourceSystem,
		Kind:      "payment_request_created",
		Data:      map[string]any{"payment_request_id": request.ID, "status": request.Status.String()},
		DedupeKey: "payment_request_created:" + request.ID,
	})
	if err != nil {
		g.markFailed(ctx, model.ID, "could not save the payment request")
		return InitiateResponse{}, err
	}
	return redirectResponse(model), nil
}

func (g *instamojoGateway) HandleWebhook(ctx context.Context, req WebhookRequest) (payment Payment, err error) {
	defer recoverAsError(&err)
	if err := g.ready(ctx); err != nil {
		return Payment{}, err
	}
	fields, err := verifyInstamojoWebhook(req.Form, g.salt)
	if err != nil {
		return Payment{}, err
	}
	requestID := fields["payment_request_id"]
	paymentID := fields["payment_id"]
	if requestID == "" || paymentID == "" {
		return Payment{}, ErrWebhookInvalid
	}
	model, err := g.payments.GetPaymentByReference(ctx, ProviderInstamojo, requestID)
	if err != nil {
		return Payment{}, err
	}
	if purpose := fields["purpose"]; purpose != "" && purpose != model.ID {
		return Payment{}, ErrWebhookInvalid
	}
	_, err = g.payments.Update(ctx, model.ID, paymentsRepository.Patch{}, paymentsRepository.EventInput{
		Source:    paymentsRepository.EventSourceWebhook,
		Kind:      "webhook_received",
		Data:      stringMap(fields),
		DedupeKey: "webhook:" + paymentID + ":" + fields["status"],
	})
	if err := ignoreDuplicate(err); err != nil {
		return Payment{}, err
	}
	model, _, err = g.applyPayment(ctx, model, paymentID, paymentsRepository.EventSourceWebhook)
	if err != nil {
		return Payment{}, err
	}
	return toPayment(model), nil
}

func (g *instamojoGateway) HandleRedirect(ctx context.Context, req RedirectRequest) (payment Payment, err error) {
	defer recoverAsError(&err)
	if err := g.ready(ctx); err != nil {
		return Payment{}, err
	}
	requestID, okRequest := cleanText(req.Query.Get("payment_request_id"), maxIDRunes)
	paymentID, okPayment := cleanText(req.Query.Get("payment_id"), maxIDRunes)
	status, okStatus := cleanText(req.Query.Get("payment_status"), maxIDRunes)
	if !okRequest || !okPayment || !okStatus || requestID == "" {
		return Payment{}, ErrRedirectInvalid
	}
	model, err := g.payments.GetPaymentByReference(ctx, ProviderInstamojo, requestID)
	if err != nil {
		return Payment{}, err
	}
	data := map[string]any{"payment_request_id": requestID, "payment_id": paymentID, "payment_status": status}
	updated, err := g.payments.Update(ctx, model.ID, paymentsRepository.Patch{Metadata: map[string]any{
		"redirect_called":     true,
		"redirect_status":     status,
		"redirect_payment_id": paymentID,
		"redirect_at":         g.now().UTC().Format(time.RFC3339Nano),
	}}, paymentsRepository.EventInput{
		Source:    paymentsRepository.EventSourceRedirect,
		Kind:      "redirect_called",
		Data:      data,
		DedupeKey: "redirect:" + paymentID + ":" + status,
	})
	switch {
	case err == nil:
		model = updated
	case errors.Is(err, paymentsRepository.ErrEventDuplicate):
	default:
		return Payment{}, err
	}
	if !model.State.IsFinal() {
		deadline := pollDeadline(model, g.now())
		if err := g.queue.enqueue(ctx, instamojoPollJob, pollMessage{PaymentID: model.ID, Deadline: deadline.UnixMicro()}, 0); err != nil {
			g.logError("could not schedule verification after redirect", "payment_id", model.ID, "error", err.Error())
		}
	}
	return toPayment(model), nil
}

func (g *instamojoGateway) Close() error {
	if g.closed.Swap(true) {
		return nil
	}
	return g.queue.close()
}

func (g *instamojoGateway) poll(ctx context.Context, body []byte) error {
	message, err := utils.FromStringToObjectJSON[pollMessage](string(body))
	if err != nil || !utils.ValidateULID(message.PaymentID) {
		g.logError("dropping malformed poll message", "body", string(body))
		return nil
	}
	model, err := g.payments.GetPayment(ctx, message.PaymentID)
	if errors.Is(err, paymentsRepository.ErrPaymentNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if model.Provider != ProviderInstamojo || model.State.IsFinal() {
		return nil
	}
	if model.GatewayReference != "" {
		model, err = g.reconcile(ctx, model, paymentsRepository.EventSourcePoll)
		if err != nil {
			return err
		}
	}
	if !model.State.IsOpen() {
		return nil
	}
	deadline := time.UnixMicro(message.Deadline)
	if now := g.now(); now.Before(deadline) {
		return g.queue.enqueue(ctx, instamojoPollJob, message, min(g.pollInterval, deadline.Sub(now)))
	}
	return g.closeWindow(ctx, model)
}

func (g *instamojoGateway) reconcile(ctx context.Context, model paymentsRepository.Model, source paymentsRepository.EventSource) (paymentsRepository.Model, error) {
	request, err := g.client.GetPaymentRequest(ctx, model.GatewayReference)
	if err != nil {
		return model, fmt.Errorf("%w: %w", ErrProviderFailed, err)
	}
	for _, link := range request.Payments {
		paymentID := lastPathSegment(link)
		if paymentID == "" {
			continue
		}
		next, done, err := g.applyPayment(ctx, model, paymentID, source)
		if err != nil {
			return model, err
		}
		model = next
		if done {
			return model, nil
		}
	}
	return model, nil
}

func (g *instamojoGateway) applyPayment(ctx context.Context, model paymentsRepository.Model, paymentID string, source paymentsRepository.EventSource) (paymentsRepository.Model, bool, error) {
	if model.State.IsFinal() {
		return model, true, nil
	}
	details, err := g.client.GetPaymentDetails(ctx, paymentID)
	if err != nil {
		return model, false, fmt.Errorf("%w: %w", ErrProviderFailed, err)
	}
	data := map[string]any{
		"payment_id":         details.ID,
		"amount":             details.Amount,
		"currency":           details.Currency,
		"instrument_type":    details.InstrumentType.String(),
		"billing_instrument": details.BillingInstrument,
		"status":             details.PaymentStatus().String(),
	}
	switch details.PaymentStatus() {
	case instamojoClient.PaymentStatusSuccessful:
		if !amountMatches(details.Amount, model.Amount) || lastPathSegment(details.PaymentRequest) != model.GatewayReference || (details.Currency != "" && details.Currency != instamojoCurrency) {
			_, err := g.payments.Update(ctx, model.ID, paymentsRepository.Patch{}, paymentsRepository.EventInput{
				Source:    source,
				Kind:      "payment_mismatch",
				Data:      data,
				DedupeKey: "mismatch:" + details.ID,
			})
			return model, false, ignoreDuplicate(err)
		}
		cleared := ""
		updated, _, err := g.transition(ctx, model.ID, StateSucceeded, paymentsRepository.Patch{
			GatewayPaymentID: &details.ID,
			FailureReason:    &cleared,
		}, paymentsRepository.EventInput{
			Source:    source,
			Kind:      "payment_succeeded",
			Data:      data,
			DedupeKey: "succeeded:" + details.ID,
		})
		if errors.Is(err, paymentsRepository.ErrEventDuplicate) || errors.Is(err, paymentsRepository.ErrTransitionNotAllowed) {
			current, loadErr := g.payments.GetPayment(ctx, model.ID)
			if loadErr != nil {
				return model, false, loadErr
			}
			return current, current.State.IsFinal(), nil
		}
		if err != nil {
			return model, false, err
		}
		return updated, true, nil
	case instamojoClient.PaymentStatusFailed:
		reason := "payment failed"
		if details.Failure != nil {
			reason = truncate(strings.TrimSpace(details.Failure.Reason+": "+details.Failure.Message), maxReasonRunes)
			data["failure_reason"] = details.Failure.Reason
			data["failure_message"] = details.Failure.Message
		}
		updated, err := g.payments.Update(ctx, model.ID, paymentsRepository.Patch{
			GatewayPaymentID: &details.ID,
			FailureReason:    &reason,
		}, paymentsRepository.EventInput{
			Source:    source,
			Kind:      "payment_failed",
			Data:      data,
			DedupeKey: "failed:" + details.ID,
		})
		if errors.Is(err, paymentsRepository.ErrEventDuplicate) {
			return model, false, nil
		}
		if err != nil {
			return model, false, err
		}
		return updated, false, nil
	default:
		return model, false, nil
	}
}

func (g *instamojoGateway) closeWindow(ctx context.Context, model paymentsRepository.Model) error {
	to := StateExpired
	reason := "payment window ended without a successful payment"
	switch {
	case model.State == StateCreated:
		to, reason = StateFailed, "payment request was never created"
	case model.FailureReason != "":
		to, reason = StateFailed, model.FailureReason
	}
	_, _, err := g.transition(ctx, model.ID, to, paymentsRepository.Patch{FailureReason: &reason}, paymentsRepository.EventInput{
		Source:    paymentsRepository.EventSourcePoll,
		Kind:      "payment_window_closed",
		Data:      map[string]any{"state": to.String()},
		DedupeKey: "payment_window_closed",
	})
	if errors.Is(err, paymentsRepository.ErrEventDuplicate) || errors.Is(err, paymentsRepository.ErrTransitionNotAllowed) {
		return nil
	}
	return err
}

func redirectResponse(model paymentsRepository.Model) InitiateResponse {
	return InitiateResponse{
		Payment: toPayment(model),
		Action:  NextAction{Type: ActionRedirect, RedirectURL: model.CheckoutURL},
	}
}

func pollDeadline(model paymentsRepository.Model, now time.Time) time.Time {
	raw, _ := model.Metadata[metadataDeadlineKey].(string)
	deadline, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil || deadline.Before(now) {
		return now
	}
	return deadline
}
