package ordersService

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	paymentGateway "github.com/thesahibnanda-max/eCommerce/backend/package/payment_gateway"
	addressRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/address"
	cartsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/carts"
	ordersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/orders"
	paymentsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/payments"
	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
	usersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/users"
	"github.com/thesahibnanda-max/eCommerce/backend/package/smtp"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	currencyINR         = "INR"
	metaStockReserved   = "stock_reserved"
	metaStockRestored   = "stock_restored"
	metaNeedsRefund     = "needs_refund"
	metaPaymentID       = "payment_id"
	metaTrackingCourier = "tracking_courier"
	metaTrackingNumber  = "tracking_number"
	metaTrackingURL     = "tracking_url"
	maxTrackingRunes    = 60
	maxTrackingURLBytes = 512
	mailTimeout         = 30 * time.Second
	shortIDLength       = 8
	maxReasonRunes      = 300
	decisionSucceeded   = "SUCCEEDED"
	decisionFailed      = "FAILED"
	reasonProofReceived = "payment proof submitted"
)

type user struct {
	id    string
	name  string
	email string
	phone string
}

func (s *service) authenticate(ctx context.Context, accessToken string) (user, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return user{}, err
	}
	verified, err := s.users.VerifyAccessToken(ctx, usersService.VerifyAccessTokenRequest{AccessToken: accessToken})
	if err != nil {
		return user{}, err
	}
	return user{id: verified.ID, name: verified.Name, email: verified.Email, phone: verified.PhoneNumber}, nil
}

func (s *service) lockUser(userID string) func() {
	value, _ := s.userLocks.LoadOrStore(userID, &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func (s *service) shippingFee(subtotal int64) int64 {
	if subtotal <= 0 || subtotal >= s.freeShippingAbove {
		return 0
	}
	return s.flatShippingFee
}

func (s *service) buildQuote(ctx context.Context, cart cartsRepository.Model) (Quote, []ordersRepository.Item, []productsRepository.StockChange, error) {
	quote := Quote{
		Lines:             []QuoteLine{},
		Problems:          []string{},
		Currency:          currencyINR,
		FreeShippingAbove: s.freeShippingAbove,
		PaymentProvider:   s.gateway.Provider().String(),
	}
	items := make([]ordersRepository.Item, 0, len(cart.Items))
	changes := make([]productsRepository.StockChange, 0, len(cart.Items))
	for _, entry := range cart.Items {
		line := QuoteLine{VariantID: entry.VariantID, Quantity: int64(entry.Quantity)}
		product, variant, err := s.products.GetProductByVariantID(ctx, entry.VariantID)
		switch {
		case errors.Is(err, productsRepository.ErrVariantNotFound):
			quote.Problems = append(quote.Problems, "An item in your bag is no longer sold.")
			quote.Lines = append(quote.Lines, line)
			continue
		case err != nil:
			return Quote{}, nil, nil, err
		}
		line.ProductID = product.ID
		line.ProductName = product.Name
		line.SKU = variant.SKU
		line.Attributes = variant.Attributes
		line.UnitPrice = variant.SellingPrice
		line.LineTotal = variant.SellingPrice * int64(entry.Quantity)
		line.StockUnits = variant.StockUnits
		line.Available = variant.StockUnits >= uint64(entry.Quantity)
		if len(variant.ImageURLs) > 0 {
			line.ImageURL = variant.ImageURLs[0]
		}
		if !strings.EqualFold(variant.Currency, currencyINR) {
			line.Available = false
			quote.Problems = append(quote.Problems, product.Name+" is not priced in INR.")
		} else if !line.Available {
			quote.Problems = append(quote.Problems, fmt.Sprintf("Only %d left of %s.", variant.StockUnits, product.Name))
		}
		quote.Lines = append(quote.Lines, line)
		quote.ItemCount += line.Quantity
		if !line.Available {
			continue
		}
		quote.Subtotal += line.LineTotal
		items = append(items, ordersRepository.Item{
			ProductID:        product.ID,
			ProductVariantID: variant.ID,
			SKU:              variant.SKU,
			ProductName:      product.Name,
			Attributes:       variant.Attributes,
			UnitPrice:        variant.SellingPrice,
			Quantity:         int64(entry.Quantity),
		})
		changes = append(changes, productsRepository.StockChange{VariantID: variant.ID, Delta: -int64(entry.Quantity)})
	}
	if len(cart.Items) == 0 {
		quote.Problems = append(quote.Problems, "Your bag is empty.")
	}
	quote.ShippingFee = s.shippingFee(quote.Subtotal)
	quote.TotalPayable = quote.Subtotal + quote.ShippingFee
	quote.Ready = len(cart.Items) > 0 && len(quote.Problems) == 0
	return quote, items, changes, nil
}

func toShippingAddress(address addressRepository.Model) ordersRepository.ShippingAddress {
	return ordersRepository.ShippingAddress{
		AddressID:    address.ID,
		FullName:     address.FullName,
		PhoneNumber:  address.PhoneNumber,
		AddressLine1: address.AddressLine1,
		AddressLine2: address.AddressLine2,
		Landmark:     address.Landmark,
		City:         address.City,
		State:        address.State,
		StateCode:    address.StateCode,
		Pincode:      address.Pincode,
		Country:      address.Country,
	}
}

func stateOf(model ordersRepository.Model) ordersRepository.OrderState {
	return model.OrderState.CurrentState().Value()
}

func latest(payments []paymentsRepository.Model) *paymentsRepository.Model {
	if len(payments) == 0 {
		return nil
	}
	return &payments[0]
}

func (s *service) orderPayments(ctx context.Context, orderID string) ([]paymentsRepository.Model, error) {
	return s.payments.ListOrderPayments(ctx, orderID)
}

func canPay(state ordersRepository.OrderState, last *paymentsRepository.Model) bool {
	if state != ordersRepository.OrderStateCreated {
		return false
	}
	return last == nil || (last.State.IsClosed() && !last.State.IsFinal())
}

func canCancel(state ordersRepository.OrderState, last *paymentsRepository.Model) bool {
	if state != ordersRepository.OrderStateCreated {
		return false
	}
	return last == nil || last.State == paymentsRepository.StateCreated || last.State == paymentsRepository.StateAwaitingPayment || (last.State.IsClosed() && !last.State.IsFinal())
}

func canSubmitProof(state ordersRepository.OrderState, last *paymentsRepository.Model) bool {
	if state != ordersRepository.OrderStateCreated && state != ordersRepository.OrderStateProcessing {
		return false
	}
	return last != nil && (last.State == paymentsRepository.StateAwaitingPayment || last.State == paymentsRepository.StateVerifying)
}

func (s *service) toOrder(ctx context.Context, model ordersRepository.Model, payments []paymentsRepository.Model, admin, detailed bool) Order {
	state := stateOf(model)
	last := latest(payments)
	order := Order{
		ID:              model.ID,
		State:           state.String(),
		Currency:        model.Currency.String(),
		Subtotal:        model.Subtotal,
		ShippingFee:     model.ShippingFee,
		Discount:        model.Discount,
		TotalPayable:    model.TotalPayable,
		Items:           make([]OrderItem, 0, len(model.Items)),
		ShippingAddress: OrderAddress(model.ShippingAddress),
		CanPay:          canPay(state, last),
		CanCancel:       canCancel(state, last),
		CanSubmitProof:  canSubmitProof(state, last),
		Tracking:        trackingOf(model.Metadata),
		Steps:           buildSteps(model),
		Closed:          isClosed(state),
		CreatedAt:       model.CreatedAt,
		UpdatedAt:       model.UpdatedAt,
	}
	for _, item := range model.Items {
		line := OrderItem{
			ProductID:        item.ProductID,
			ProductVariantID: item.ProductVariantID,
			SKU:              item.SKU,
			ProductName:      item.ProductName,
			Attributes:       item.Attributes,
			UnitPrice:        item.UnitPrice,
			Quantity:         item.Quantity,
			LineTotal:        item.LineTotal,
		}
		if _, variant, err := s.products.GetProductByVariantID(ctx, item.ProductVariantID); err == nil && len(variant.ImageURLs) > 0 {
			line.ImageURL = variant.ImageURLs[0]
		}
		order.ItemCount += item.Quantity
		order.Items = append(order.Items, line)
	}
	if last != nil {
		payment := toPayment(*last, admin)
		order.Payment = &payment
	}
	if detailed {
		order.History = make([]StateChange, 0, len(model.StateHistory))
		for _, change := range model.StateHistory {
			order.History = append(order.History, StateChange{
				FromState: change.FromState.String(),
				ToState:   change.ToState.String(),
				Reason:    change.Reason,
				Actor:     change.Actor.String(),
				CreatedAt: change.CreatedAt,
			})
		}
	}
	if admin {
		order.UserID = model.UserID
		order.Metadata = model.Metadata
		order.AllowedTransitions = []string{}
		for _, next := range ordersRepository.AllowedTransitions(state) {
			order.AllowedTransitions = append(order.AllowedTransitions, next.String())
		}
		if detailed {
			order.Payments = make([]Payment, 0, len(payments))
			for _, payment := range payments {
				order.Payments = append(order.Payments, toPayment(payment, true))
			}
		}
	}
	return order
}

func toPayment(model paymentsRepository.Model, admin bool) Payment {
	payment := Payment{
		ID:            model.ID,
		OrderID:       model.OrderID,
		Provider:      model.Provider.String(),
		State:         model.State.String(),
		Amount:        model.Amount,
		Currency:      model.Currency.String(),
		CheckoutURL:   model.CheckoutURL,
		FailureReason: model.FailureReason,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
	}
	if model.Proof != nil {
		submitted := model.Proof.SubmittedAt
		payment.ProofTransactionID = model.Proof.TransactionID
		payment.ProofSubmittedAt = &submitted
		payment.HasProofImage = true
	}
	if admin {
		payment.UserID = model.UserID
		payment.GatewayReference = model.GatewayReference
		payment.GatewayPaymentID = model.GatewayPaymentID
		payment.Metadata = model.Metadata
		payment.Events = make([]PaymentEvent, 0, len(model.Events))
		for _, event := range model.Events {
			payment.Events = append(payment.Events, PaymentEvent{
				Source:    event.Source.String(),
				Kind:      event.Kind,
				FromState: event.FromState.String(),
				ToState:   event.ToState.String(),
				Data:      event.Data,
				CreatedAt: event.CreatedAt,
			})
		}
	}
	return payment
}

func toAction(action paymentGateway.NextAction) *PaymentAction {
	return &PaymentAction{
		Type:           action.Type.String(),
		RedirectURL:    action.RedirectURL,
		QRImageDataURI: action.QRImageDataURI,
		UPIID:          action.UPIID,
		UPIIntentURL:   action.UPIIntentURL,
	}
}

func (s *service) initiatePayment(ctx context.Context, model ordersRepository.Model, buyer user) (*PaymentAction, error) {
	resp, err := s.gateway.InitiatePayment(ctx, paymentGateway.InitiateRequest{
		OrderID:   model.ID,
		UserID:    model.UserID,
		Amount:    model.TotalPayable,
		Purpose:   "Order " + shortID(model.ID),
		BuyerName: buyer.name,
		Email:     buyer.email,
		Phone:     buyer.phone,
	})
	if err != nil {
		return nil, err
	}
	if model.PaymentGateway == "" {
		if _, err := s.orders.SetPaymentGateway(ctx, model.ID, s.gateway.Provider().String()); err != nil && !errors.Is(err, ordersRepository.ErrPaymentGatewayLocked) {
			return nil, err
		}
	}
	if _, err := s.orders.UpdateMetadata(ctx, model.ID, func(m map[string]any) (map[string]any, error) {
		m[metaPaymentID] = resp.Payment.ID
		return m, nil
	}); err != nil {
		return nil, err
	}
	return toAction(resp.Action), nil
}

func (s *service) currentAction(ctx context.Context, model ordersRepository.Model, last *paymentsRepository.Model) *PaymentAction {
	if last == nil || last.State != paymentsRepository.StateAwaitingPayment || stateOf(model) != ordersRepository.OrderStateCreated {
		return nil
	}
	resp, err := s.gateway.InitiatePayment(ctx, paymentGateway.InitiateRequest{
		OrderID: model.ID,
		UserID:  model.UserID,
		Amount:  model.TotalPayable,
		Purpose: "Order " + shortID(model.ID),
	})
	if err != nil {
		s.logError("could not load payment instructions", "order_id", model.ID, "error", err.Error())
		return nil
	}
	return toAction(resp.Action)
}

func (s *service) restoreStock(ctx context.Context, model ordersRepository.Model) error {
	_, err := s.orders.UpdateMetadata(ctx, model.ID, func(m map[string]any) (map[string]any, error) {
		if m[metaStockReserved] != true || m[metaStockRestored] == true {
			return m, nil
		}
		changes := make([]productsRepository.StockChange, 0, len(model.Items))
		for _, item := range model.Items {
			if _, _, err := s.products.GetProductByVariantID(ctx, item.ProductVariantID); err != nil {
				continue
			}
			changes = append(changes, productsRepository.StockChange{VariantID: item.ProductVariantID, Delta: item.Quantity})
		}
		if len(changes) > 0 {
			if _, err := s.products.AdjustStock(ctx, changes); err != nil {
				return nil, err
			}
		}
		m[metaStockRestored] = true
		return m, nil
	})
	return err
}

func releasesStock(state ordersRepository.OrderState) bool {
	return state == ordersRepository.OrderStateCancelled || state == ordersRepository.OrderStateFailed || state == ordersRepository.OrderStateDitched
}

func (s *service) transition(ctx context.Context, model ordersRepository.Model, to ordersRepository.OrderState, actor ordersRepository.Actor, reason string) (ordersRepository.Model, error) {
	updated, err := s.orders.TransitionOrder(ctx, model.ID, to, actor, truncate(reason, maxReasonRunes))
	if err != nil {
		return model, err
	}
	if releasesStock(to) {
		if err := s.restoreStock(ctx, updated); err != nil {
			s.logError("could not restore stock", "order_id", model.ID, "error", err.Error())
		}
	}
	return updated, nil
}

func (s *service) advanceToApproved(ctx context.Context, model ordersRepository.Model, actor ordersRepository.Actor) (ordersRepository.Model, bool, error) {
	var err error
	switch stateOf(model) {
	case ordersRepository.OrderStateCreated:
		if model, err = s.transition(ctx, model, ordersRepository.OrderStateProcessing, actor, "payment received"); err != nil {
			return model, false, err
		}
		fallthrough
	case ordersRepository.OrderStateProcessing, ordersRepository.OrderStateOnHold:
		model, err = s.transition(ctx, model, ordersRepository.OrderStateApproved, actor, "payment confirmed")
		return model, err == nil, err
	default:
		return model, false, nil
	}
}

func (s *service) closeOpenPayment(ctx context.Context, orderID, reason string) error {
	payments, err := s.orderPayments(ctx, orderID)
	if err != nil {
		return err
	}
	last := latest(payments)
	if last == nil || !last.State.IsOpen() {
		return nil
	}
	if last.State == paymentsRepository.StateVerifying {
		return ErrOrderCannotCancel
	}
	reason = truncate(reason, maxReasonRunes)
	_, _, err = s.payments.Transition(ctx, last.ID, paymentsRepository.StateFailed, paymentsRepository.Patch{FailureReason: &reason}, paymentsRepository.EventInput{
		Source: paymentsRepository.EventSourceSystem,
		Kind:   "order_cancelled",
		Data:   map[string]any{"reason": reason},
	})
	if errors.Is(err, paymentsRepository.ErrTransitionNotAllowed) {
		return ErrOrderCannotCancel
	}
	return err
}

func (s *service) recordPaymentEvent(ctx context.Context, orderID string, payment paymentGateway.Payment) {
	_, err := s.orders.RecordPaymentEvent(ctx, orderID, ordersRepository.PaymentEvent{
		Gateway:          payment.Provider.String(),
		GatewayEventID:   payment.ID + ":" + payment.State.String(),
		EventType:        "payment_" + strings.ToLower(payment.State.String()),
		GatewayOrderID:   payment.GatewayReference,
		GatewayPaymentID: payment.GatewayPaymentID,
		Status:           payment.State.String(),
		Amount:           payment.Amount,
		Raw:              map[string]any{"payment_id": payment.ID, "failure_reason": payment.FailureReason, "proof_transaction_id": payment.ProofTransactionID},
	})
	if err != nil && !errors.Is(err, ordersRepository.ErrPaymentEventDuplicate) {
		s.logError("could not record payment event on order", "order_id", orderID, "error", err.Error())
	}
}

func actorFor(provider paymentGateway.Provider) ordersRepository.Actor {
	if provider == paymentGateway.ProviderSelfUPI {
		return ordersRepository.ActorAdmin
	}
	return ordersRepository.ActorGateway
}

func (s *service) notify(model ordersRepository.Model, subject, headline, detail string) {
	s.mailWG.Add(1)
	go func() {
		defer s.mailWG.Done()
		defer func() {
			if recovered := recover(); recovered != nil {
				s.logError("order email panicked", "order_id", model.ID, "panic", fmt.Sprint(recovered))
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), mailTimeout)
		defer cancel()
		recipient, err := s.usersRepo.GetUserByID(ctx, model.UserID)
		if err != nil || recipient.Email == "" {
			s.logError("order email skipped: no recipient", "order_id", model.ID)
			return
		}
		link := s.storeURL + "/orders/" + model.ID
		name := recipient.Name
		if name == "" {
			name = "there"
		}
		text := fmt.Sprintf("Hi %s,\n\n%s\n%s\n\nOrder %s, total ₹%d.\nTrack it here: %s\n\n%s", name, headline, detail, shortID(model.ID), model.TotalPayable, link, s.storeName)
		htmlBody := fmt.Sprintf(`<p>Hi %s,</p><p><strong>%s</strong></p><p>%s</p><p>Order <strong>%s</strong>, total <strong>₹%d</strong>.</p><p><a href="%s">View your order</a></p><p>%s</p>`,
			html.EscapeString(name), html.EscapeString(headline), html.EscapeString(detail), shortID(model.ID), model.TotalPayable, html.EscapeString(link), html.EscapeString(s.storeName))
		if err := s.mailer.SendMail(ctx, smtp.SendMailRequest{To: recipient.Email, Subject: subject, Body: text, HTMLBody: htmlBody}); err != nil {
			s.logError("order email failed", "order_id", model.ID, "error", err.Error())
		}
	}()
}

func (s *service) notifyState(model ordersRepository.Model, to ordersRepository.OrderState, reason string) {
	ref := shortID(model.ID)
	switch to {
	case ordersRepository.OrderStatePacked:
		s.notify(model, "Your order "+ref+" is packed", "Your order is packed and ready to ship.", "We'll send tracking details as soon as it leaves us.")
	case ordersRepository.OrderStateStartedShipping:
		s.notify(model, "Your order "+ref+" has shipped", "Your order is on its way!", trackingLine(model.Metadata, "We'll let you know once it's out for delivery."))
	case ordersRepository.OrderStateOutForDelivery:
		s.notify(model, "Your order "+ref+" is out for delivery", "Your order is out for delivery today.", trackingLine(model.Metadata, "Please keep your phone handy for the delivery partner."))
	case ordersRepository.OrderStateDelivered:
		s.notify(model, "Your order "+ref+" was delivered", "Your order has been delivered.", "We hope you love it.")
	case ordersRepository.OrderStateCancelled:
		s.notify(model, "Your order "+ref+" was cancelled", "Your order has been cancelled.", reasonOrDefault(reason, "If you already paid, we'll refund you."))
	case ordersRepository.OrderStateRefunded:
		s.notify(model, "Refund for order "+ref, "Your refund has been processed.", reasonOrDefault(reason, "It can take a few days to show in your account."))
	}
}

func trackingLine(metadata map[string]any, fallback string) string {
	tracking := trackingOf(metadata)
	if tracking == nil {
		return fallback
	}
	return fmt.Sprintf("Courier: %s, tracking number %s. Track it here: %s", tracking.Courier, tracking.Number, tracking.URL)
}

func trackingOf(metadata map[string]any) *Tracking {
	url, _ := metadata[metaTrackingURL].(string)
	if url == "" {
		return nil
	}
	courier, _ := metadata[metaTrackingCourier].(string)
	number, _ := metadata[metaTrackingNumber].(string)
	return &Tracking{Courier: courier, Number: number, URL: url}
}

func normalizeTracking(request AdminSetTrackingRequest) (*Tracking, error) {
	courier := strings.TrimSpace(request.Courier)
	number := strings.TrimSpace(request.Number)
	link := strings.TrimSpace(request.URL)
	if courier == "" && number == "" && link == "" {
		return nil, nil
	}
	if courier == "" || number == "" || utf8.RuneCountInString(courier) > maxTrackingRunes || utf8.RuneCountInString(number) > maxTrackingRunes || len(link) > maxTrackingURLBytes {
		return nil, ErrTrackingInvalid
	}
	parsed, err := url.Parse(link)
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return nil, ErrTrackingInvalid
	}
	return &Tracking{Courier: courier, Number: number, URL: link}, nil
}

type stepDefinition struct {
	key   string
	label string
	state ordersRepository.OrderState
}

var deliverySteps = []stepDefinition{
	{"placed", "Order placed", ordersRepository.OrderStateCreated},
	{"confirmed", "Confirmed", ordersRepository.OrderStateApproved},
	{"packed", "Packed", ordersRepository.OrderStatePacked},
	{"shipped", "Shipped", ordersRepository.OrderStateStartedShipping},
	{"out_for_delivery", "Out for delivery", ordersRepository.OrderStateOutForDelivery},
	{"delivered", "Delivered", ordersRepository.OrderStateDelivered},
}

func stepRank(state ordersRepository.OrderState) int {
	for i, step := range deliverySteps {
		if step.state == state {
			return i
		}
	}
	return -1
}

func isClosed(state ordersRepository.OrderState) bool {
	switch state {
	case ordersRepository.OrderStateCancelled, ordersRepository.OrderStateFailed, ordersRepository.OrderStateDitched, ordersRepository.OrderStateReturned, ordersRepository.OrderStateRefunded:
		return true
	default:
		return false
	}
}

func buildSteps(model ordersRepository.Model) []Step {
	reached := map[ordersRepository.OrderState]time.Time{ordersRepository.OrderStateCreated: model.CreatedAt}
	highest := 0
	for _, change := range model.StateHistory {
		if _, seen := reached[change.ToState]; !seen {
			reached[change.ToState] = change.CreatedAt
		}
		if rank := stepRank(change.ToState); rank > highest {
			highest = rank
		}
	}
	steps := make([]Step, 0, len(deliverySteps))
	for i, definition := range deliverySteps {
		step := Step{Key: definition.key, Label: definition.label, Done: i <= highest, Current: i == highest && !isClosed(stateOf(model))}
		if at, ok := reached[definition.state]; ok && step.Done {
			value := at
			step.At = &value
		}
		steps = append(steps, step)
	}
	return steps
}

func reasonOrDefault(reason, fallback string) string {
	if strings.TrimSpace(reason) == "" {
		return fallback
	}
	return reason
}

func (s *service) logError(msg string, args ...any) {
	if s.logger != nil {
		s.logger.Error(msg, args...)
	}
}

func shortID(id string) string {
	if len(id) < 26 {
		return strings.ToUpper(id)
	}
	return strings.ToUpper(id[26-shortIDLength : 26])
}

func truncate(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}

func parseOrderStates(values []string) ([]ordersRepository.OrderState, error) {
	states := make([]ordersRepository.OrderState, 0, len(values))
	for _, value := range values {
		state, err := ordersRepository.ParseOrderState(strings.ToUpper(strings.TrimSpace(value)))
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return states, nil
}

func parsePaymentStates(values []string) ([]paymentsRepository.State, error) {
	states := make([]paymentsRepository.State, 0, len(values))
	for _, value := range values {
		state := paymentsRepository.State(strings.ToUpper(strings.TrimSpace(value)))
		if !state.IsValid() {
			return nil, paymentsRepository.ErrStateInvalid
		}
		states = append(states, state)
	}
	return states, nil
}

func formatRupees(amount int64) string {
	return strconv.FormatInt(amount, 10)
}
