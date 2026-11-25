package ordersService

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"

	paymentGateway "github.com/thesahibnanda-max/eCommerce/backend/package/payment_gateway"
	addressRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/address"
	cartsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/carts"
	ordersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/orders"
	paymentsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/payments"
	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
	usersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/users"
	usersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/users"
	"github.com/thesahibnanda-max/eCommerce/backend/package/smtp"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

type Service interface {
	Quote(ctx context.Context, request QuoteRequest) (QuoteResponse, error)
	PlaceOrder(ctx context.Context, request PlaceOrderRequest) (OrderResponse, error)
	ListOrders(ctx context.Context, request ListOrdersRequest) (ListOrdersResponse, error)
	GetOrder(ctx context.Context, request GetOrderRequest) (OrderResponse, error)
	PayOrder(ctx context.Context, request PayOrderRequest) (OrderResponse, error)
	SubmitPaymentProof(ctx context.Context, request SubmitPaymentProofRequest) (OrderResponse, error)
	CancelOrder(ctx context.Context, request CancelOrderRequest) (OrderResponse, error)
	HandleProviderWebhook(ctx context.Context, request ProviderWebhookRequest) (ProviderCallbackResponse, error)
	HandleProviderRedirect(ctx context.Context, request ProviderRedirectRequest) (ProviderCallbackResponse, error)
	HandlePaymentFinalized(ctx context.Context, payment paymentGateway.Payment)
	AdminListOrders(ctx context.Context, request AdminListOrdersRequest) (ListOrdersResponse, error)
	AdminGetOrder(ctx context.Context, orderID string) (OrderResponse, error)
	AdminTransitionOrder(ctx context.Context, request AdminTransitionOrderRequest) (OrderResponse, error)
	AdminSetTracking(ctx context.Context, request AdminSetTrackingRequest) (OrderResponse, error)
	AdminListPayments(ctx context.Context, request AdminListPaymentsRequest) (ListPaymentsResponse, error)
	AdminGetPayment(ctx context.Context, paymentID string) (PaymentResponse, error)
	AdminDecidePayment(ctx context.Context, request AdminDecidePaymentRequest) (PaymentResponse, error)
	AdminListUserOrders(ctx context.Context, userID string) (ListOrdersResponse, error)
	Wait()
}

type Params struct {
	Users             usersService.Service
	UsersRepo         usersRepository.Repository
	Products          productsRepository.Repository
	Carts             cartsRepository.Repository
	Addresses         addressRepository.Repository
	Orders            ordersRepository.Repository
	Payments          paymentsRepository.Repository
	Gateway           paymentGateway.Gateway
	Mailer            smtp.Mailer
	FreeShippingAbove int64
	ShippingFee       int64
	StoreName         string
	StoreURL          string
	Logger            *slog.Logger
}

type service struct {
	users             usersService.Service
	usersRepo         usersRepository.Repository
	products          productsRepository.Repository
	carts             cartsRepository.Repository
	addresses         addressRepository.Repository
	orders            ordersRepository.Repository
	payments          paymentsRepository.Repository
	gateway           paymentGateway.Gateway
	mailer            smtp.Mailer
	freeShippingAbove int64
	flatShippingFee   int64
	storeName         string
	storeURL          string
	logger            *slog.Logger
	userLocks         sync.Map
	mailWG            sync.WaitGroup
}

func NewService(p Params) (Service, error) {
	if p.Users == nil || p.UsersRepo == nil || p.Products == nil || p.Carts == nil || p.Addresses == nil || p.Orders == nil || p.Payments == nil || p.Gateway == nil || p.Mailer == nil {
		return nil, ErrDependencyMissing
	}
	if p.FreeShippingAbove < 0 || p.ShippingFee < 0 {
		return nil, ErrPolicyInvalid
	}
	return &service{
		users:             p.Users,
		usersRepo:         p.UsersRepo,
		products:          p.Products,
		carts:             p.Carts,
		addresses:         p.Addresses,
		orders:            p.Orders,
		payments:          p.Payments,
		gateway:           p.Gateway,
		mailer:            p.Mailer,
		freeShippingAbove: p.FreeShippingAbove,
		flatShippingFee:   p.ShippingFee,
		storeName:         strings.TrimSpace(p.StoreName),
		storeURL:          strings.TrimRight(strings.TrimSpace(p.StoreURL), "/"),
		logger:            p.Logger,
	}, nil
}

func (s *service) Wait() {
	s.mailWG.Wait()
}

func (s *service) Quote(ctx context.Context, request QuoteRequest) (QuoteResponse, error) {
	buyer, err := s.authenticate(ctx, request.AccessToken)
	if err != nil {
		return QuoteResponse{}, err
	}
	cart, err := s.carts.GetCart(ctx, buyer.id)
	if err != nil {
		return QuoteResponse{}, err
	}
	quote, _, _, err := s.buildQuote(ctx, cart)
	if err != nil {
		return QuoteResponse{}, err
	}
	return QuoteResponse{Quote: quote}, nil
}

func (s *service) PlaceOrder(ctx context.Context, request PlaceOrderRequest) (OrderResponse, error) {
	buyer, err := s.authenticate(ctx, request.AccessToken)
	if err != nil {
		return OrderResponse{}, err
	}
	addressID := strings.TrimSpace(request.AddressID)
	if addressID == "" {
		return OrderResponse{}, ErrAddressRequired
	}
	unlock := s.lockUser(buyer.id)
	defer unlock()

	existing, err := s.orders.GetUserOrderByIdempotencyKey(ctx, buyer.id, request.IdempotencyKey)
	switch {
	case err == nil:
		return s.orderResponse(ctx, existing)
	case !errors.Is(err, ordersRepository.ErrOrderNotFound):
		return OrderResponse{}, err
	}
	address, err := s.addresses.GetAddress(ctx, buyer.id, addressID)
	if err != nil {
		return OrderResponse{}, err
	}
	cart, err := s.carts.GetCart(ctx, buyer.id)
	if err != nil {
		return OrderResponse{}, err
	}
	if len(cart.Items) == 0 {
		return OrderResponse{}, ErrCartEmpty
	}
	quote, items, changes, err := s.buildQuote(ctx, cart)
	if err != nil {
		return OrderResponse{}, err
	}
	if !quote.Ready {
		for _, line := range quote.Lines {
			if line.ProductID == "" {
				return OrderResponse{}, ErrItemUnavailable
			}
		}
		for _, line := range quote.Lines {
			if !line.Available && line.StockUnits < uint64(line.Quantity) {
				return OrderResponse{}, ErrOutOfStock
			}
		}
		return OrderResponse{}, ErrCurrencyUnsupported
	}
	if _, err := s.products.AdjustStock(ctx, changes); err != nil {
		if errors.Is(err, productsRepository.ErrInsufficientStock) {
			return OrderResponse{}, ErrOutOfStock
		}
		return OrderResponse{}, err
	}
	model, err := s.orders.CreateOrder(ctx, ordersRepository.CreateOrderInput{
		UserID:          buyer.id,
		IdempotencyKey:  request.IdempotencyKey,
		Currency:        ordersRepository.IndianNationalRupeeCurrency,
		ShippingFee:     quote.ShippingFee,
		ShippingAddress: toShippingAddress(address),
		Items:           items,
		Metadata:        map[string]any{metaStockReserved: true},
	})
	if err != nil {
		restore := make([]productsRepository.StockChange, 0, len(changes))
		for _, change := range changes {
			restore = append(restore, productsRepository.StockChange{VariantID: change.VariantID, Delta: -change.Delta})
		}
		if _, restoreErr := s.products.AdjustStock(context.WithoutCancel(ctx), restore); restoreErr != nil {
			s.logError("could not release stock after failed order", "user_id", buyer.id, "error", restoreErr.Error())
		}
		return OrderResponse{}, err
	}
	if err := s.carts.DeleteCart(ctx, buyer.id); err != nil {
		s.logError("could not clear cart after order", "order_id", model.ID, "error", err.Error())
	}
	action, err := s.initiatePayment(ctx, model, buyer)
	if err != nil {
		s.logError("payment could not start, customer can retry", "order_id", model.ID, "error", err.Error())
	}
	response, err := s.orderResponse(ctx, model)
	if err != nil {
		return OrderResponse{}, err
	}
	if action != nil && response.Order.Payment != nil {
		response.Order.Payment.Action = action
	}
	return response, nil
}

func (s *service) ListOrders(ctx context.Context, request ListOrdersRequest) (ListOrdersResponse, error) {
	buyer, err := s.authenticate(ctx, request.AccessToken)
	if err != nil {
		return ListOrdersResponse{}, err
	}
	states, err := parseOrderStates(request.States)
	if err != nil {
		return ListOrdersResponse{}, err
	}
	page, err := s.orders.ListUserOrders(ctx, buyer.id, ordersRepository.ListQuery{States: states, Limit: request.Limit, Cursor: request.Cursor})
	if err != nil {
		return ListOrdersResponse{}, err
	}
	return s.listResponse(ctx, page, false)
}

func (s *service) GetOrder(ctx context.Context, request GetOrderRequest) (OrderResponse, error) {
	buyer, err := s.authenticate(ctx, request.AccessToken)
	if err != nil {
		return OrderResponse{}, err
	}
	model, err := s.orders.GetUserOrder(ctx, buyer.id, strings.TrimSpace(request.OrderID))
	if err != nil {
		return OrderResponse{}, err
	}
	return s.orderResponse(ctx, model)
}

func (s *service) PayOrder(ctx context.Context, request PayOrderRequest) (OrderResponse, error) {
	buyer, err := s.authenticate(ctx, request.AccessToken)
	if err != nil {
		return OrderResponse{}, err
	}
	unlock := s.lockUser(buyer.id)
	defer unlock()
	model, err := s.orders.GetUserOrder(ctx, buyer.id, strings.TrimSpace(request.OrderID))
	if err != nil {
		return OrderResponse{}, err
	}
	payments, err := s.orderPayments(ctx, model.ID)
	if err != nil {
		return OrderResponse{}, err
	}
	last := latest(payments)
	if !canPay(stateOf(model), last) && (last == nil || !last.State.IsOpen()) {
		return OrderResponse{}, ErrOrderCannotPay
	}
	action, err := s.initiatePayment(ctx, model, buyer)
	if err != nil {
		return OrderResponse{}, err
	}
	response, err := s.orderResponse(ctx, model)
	if err != nil {
		return OrderResponse{}, err
	}
	if response.Order.Payment != nil {
		response.Order.Payment.Action = action
	}
	return response, nil
}

func (s *service) SubmitPaymentProof(ctx context.Context, request SubmitPaymentProofRequest) (OrderResponse, error) {
	buyer, err := s.authenticate(ctx, request.AccessToken)
	if err != nil {
		return OrderResponse{}, err
	}
	model, err := s.orders.GetUserOrder(ctx, buyer.id, strings.TrimSpace(request.OrderID))
	if err != nil {
		return OrderResponse{}, err
	}
	payments, err := s.orderPayments(ctx, model.ID)
	if err != nil {
		return OrderResponse{}, err
	}
	last := latest(payments)
	if !canSubmitProof(stateOf(model), last) {
		return OrderResponse{}, ErrProofNotAllowed
	}
	if _, err := s.gateway.SubmitProof(ctx, paymentGateway.SubmitProofRequest{
		PaymentID:     last.ID,
		UserID:        buyer.id,
		TransactionID: request.TransactionID,
		Image:         request.Image,
	}); err != nil {
		return OrderResponse{}, err
	}
	if stateOf(model) == ordersRepository.OrderStateCreated {
		if model, err = s.transition(ctx, model, ordersRepository.OrderStateProcessing, ordersRepository.ActorUser, reasonProofReceived); err != nil && !errors.Is(err, ordersRepository.ErrTransitionNotAllowed) {
			return OrderResponse{}, err
		}
	}
	model, err = s.orders.GetUserOrder(ctx, buyer.id, model.ID)
	if err != nil {
		return OrderResponse{}, err
	}
	return s.orderResponse(ctx, model)
}

func (s *service) CancelOrder(ctx context.Context, request CancelOrderRequest) (OrderResponse, error) {
	buyer, err := s.authenticate(ctx, request.AccessToken)
	if err != nil {
		return OrderResponse{}, err
	}
	unlock := s.lockUser(buyer.id)
	defer unlock()
	model, err := s.orders.GetUserOrder(ctx, buyer.id, strings.TrimSpace(request.OrderID))
	if err != nil {
		return OrderResponse{}, err
	}
	payments, err := s.orderPayments(ctx, model.ID)
	if err != nil {
		return OrderResponse{}, err
	}
	if !canCancel(stateOf(model), latest(payments)) {
		return OrderResponse{}, ErrOrderCannotCancel
	}
	if err := s.closeOpenPayment(ctx, model.ID, "order cancelled by customer"); err != nil {
		return OrderResponse{}, err
	}
	model, err = s.transition(ctx, model, ordersRepository.OrderStateCancelled, ordersRepository.ActorUser, "cancelled by customer")
	if errors.Is(err, ordersRepository.ErrTransitionNotAllowed) {
		return OrderResponse{}, ErrOrderCannotCancel
	}
	if err != nil {
		return OrderResponse{}, err
	}
	return s.orderResponse(ctx, model)
}

func (s *service) HandleProviderWebhook(ctx context.Context, request ProviderWebhookRequest) (ProviderCallbackResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return ProviderCallbackResponse{}, err
	}
	if s.gateway.Provider() == paymentGateway.ProviderSelfUPI {
		return ProviderCallbackResponse{}, ErrNotSupported
	}
	payment, err := s.gateway.HandleWebhook(ctx, paymentGateway.WebhookRequest{Form: request.Form})
	if err != nil {
		return ProviderCallbackResponse{}, err
	}
	return ProviderCallbackResponse{OrderID: payment.OrderID, PaymentID: payment.ID, PaymentState: payment.State.String()}, nil
}

func (s *service) HandleProviderRedirect(ctx context.Context, request ProviderRedirectRequest) (ProviderCallbackResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return ProviderCallbackResponse{}, err
	}
	payment, err := s.gateway.HandleRedirect(ctx, paymentGateway.RedirectRequest{Query: request.Query})
	if err != nil {
		return ProviderCallbackResponse{}, err
	}
	return ProviderCallbackResponse{OrderID: payment.OrderID, PaymentID: payment.ID, PaymentState: payment.State.String()}, nil
}

func (s *service) HandlePaymentFinalized(ctx context.Context, payment paymentGateway.Payment) {
	model, err := s.orders.GetOrder(ctx, payment.OrderID)
	if err != nil {
		s.logError("finalized payment has no order", "payment_id", payment.ID, "error", err.Error())
		return
	}
	s.recordPaymentEvent(ctx, model.ID, payment)
	actor := actorFor(payment.Provider)
	ref := shortID(model.ID)
	switch payment.State {
	case paymentGateway.StateSucceeded:
		updated, approved, err := s.advanceToApproved(ctx, model, actor)
		if err != nil {
			s.logError("could not approve order after payment", "order_id", model.ID, "error", err.Error())
		}
		if !approved && err == nil {
			if _, err := s.orders.UpdateMetadata(ctx, model.ID, func(m map[string]any) (map[string]any, error) {
				m[metaNeedsRefund] = true
				return m, nil
			}); err != nil {
				s.logError("could not flag order for refund", "order_id", model.ID, "error", err.Error())
			}
		}
		s.notify(updated, "Payment received for order "+ref, "We've received your payment of ₹"+formatRupees(payment.Amount)+".", "Your order is confirmed and we're getting it ready.")
	case paymentGateway.StateFailed, paymentGateway.StateExpired:
		state := stateOf(model)
		if state != ordersRepository.OrderStateCreated && state != ordersRepository.OrderStateProcessing {
			return
		}
		reason := reasonOrDefault(payment.FailureReason, "payment was not completed")
		updated, err := s.transition(ctx, model, ordersRepository.OrderStateFailed, actor, reason)
		if err != nil {
			s.logError("could not fail order after payment", "order_id", model.ID, "error", err.Error())
			return
		}
		s.notify(updated, "Payment not confirmed for order "+ref, "We couldn't confirm your payment.", "Reason: "+reason+". If money left your account, reply to this email and we'll sort it out.")
	}
}

func (s *service) AdminListOrders(ctx context.Context, request AdminListOrdersRequest) (ListOrdersResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return ListOrdersResponse{}, err
	}
	states, err := parseOrderStates(request.States)
	if err != nil {
		return ListOrdersResponse{}, err
	}
	page, err := s.orders.ListOrders(ctx, ordersRepository.ListQuery{States: states, Limit: request.Limit, Cursor: request.Cursor})
	if err != nil {
		return ListOrdersResponse{}, err
	}
	return s.listResponse(ctx, page, true)
}

func (s *service) AdminListUserOrders(ctx context.Context, userID string) (ListOrdersResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return ListOrdersResponse{}, err
	}
	page, err := s.orders.ListUserOrders(ctx, strings.TrimSpace(userID), ordersRepository.ListQuery{Limit: 100})
	if err != nil {
		return ListOrdersResponse{}, err
	}
	return s.listResponse(ctx, page, true)
}

func (s *service) AdminGetOrder(ctx context.Context, orderID string) (OrderResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return OrderResponse{}, err
	}
	model, err := s.orders.GetOrder(ctx, strings.TrimSpace(orderID))
	if err != nil {
		return OrderResponse{}, err
	}
	return s.adminOrderResponse(ctx, model)
}

func (s *service) AdminTransitionOrder(ctx context.Context, request AdminTransitionOrderRequest) (OrderResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return OrderResponse{}, err
	}
	to, err := ordersRepository.ParseOrderState(strings.ToUpper(strings.TrimSpace(request.To)))
	if err != nil {
		return OrderResponse{}, err
	}
	model, err := s.orders.GetOrder(ctx, strings.TrimSpace(request.OrderID))
	if err != nil {
		return OrderResponse{}, err
	}
	if to == ordersRepository.OrderStateCancelled || to == ordersRepository.OrderStateFailed || to == ordersRepository.OrderStateDitched {
		if err := s.closeOpenPayment(ctx, model.ID, reasonOrDefault(request.Reason, "order closed by admin")); err != nil && !errors.Is(err, ErrOrderCannotCancel) {
			return OrderResponse{}, err
		}
	}
	model, err = s.transition(ctx, model, to, ordersRepository.ActorAdmin, reasonOrDefault(request.Reason, "updated by admin"))
	if err != nil {
		return OrderResponse{}, err
	}
	s.notifyState(model, to, request.Reason)
	return s.adminOrderResponse(ctx, model)
}

func (s *service) AdminSetTracking(ctx context.Context, request AdminSetTrackingRequest) (OrderResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return OrderResponse{}, err
	}
	tracking, err := normalizeTracking(request)
	if err != nil {
		return OrderResponse{}, err
	}
	model, err := s.orders.UpdateMetadata(ctx, strings.TrimSpace(request.OrderID), func(m map[string]any) (map[string]any, error) {
		if tracking == nil {
			delete(m, metaTrackingCourier)
			delete(m, metaTrackingNumber)
			delete(m, metaTrackingURL)
			return m, nil
		}
		m[metaTrackingCourier] = tracking.Courier
		m[metaTrackingNumber] = tracking.Number
		m[metaTrackingURL] = tracking.URL
		return m, nil
	})
	if err != nil {
		return OrderResponse{}, err
	}
	return s.adminOrderResponse(ctx, model)
}

func (s *service) AdminListPayments(ctx context.Context, request AdminListPaymentsRequest) (ListPaymentsResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return ListPaymentsResponse{}, err
	}
	states, err := parsePaymentStates(request.States)
	if err != nil {
		return ListPaymentsResponse{}, err
	}
	page, err := s.payments.ListPayments(ctx, paymentsRepository.ListQuery{
		States:   states,
		Provider: paymentsRepository.Provider(strings.ToUpper(strings.TrimSpace(request.Provider))),
		Limit:    request.Limit,
		Cursor:   request.Cursor,
	})
	if err != nil {
		return ListPaymentsResponse{}, err
	}
	response := ListPaymentsResponse{Payments: make([]Payment, 0, len(page.Payments)), NextCursor: page.NextCursor}
	for _, payment := range page.Payments {
		response.Payments = append(response.Payments, toPayment(payment, true))
	}
	return response, nil
}

func (s *service) AdminGetPayment(ctx context.Context, paymentID string) (PaymentResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return PaymentResponse{}, err
	}
	payment, err := s.payments.GetPayment(ctx, strings.TrimSpace(paymentID))
	if err != nil {
		return PaymentResponse{}, err
	}
	return PaymentResponse{Payment: toPayment(payment, true)}, nil
}

func (s *service) AdminDecidePayment(ctx context.Context, request AdminDecidePaymentRequest) (PaymentResponse, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return PaymentResponse{}, err
	}
	status := strings.ToUpper(strings.TrimSpace(request.Status))
	if status != decisionSucceeded && status != decisionFailed {
		return PaymentResponse{}, ErrPaymentDecisionInvalid
	}
	if s.gateway.Provider() != paymentGateway.ProviderSelfUPI {
		return PaymentResponse{}, ErrNotSupported
	}
	form := map[string][]string{"payment_id": {strings.TrimSpace(request.PaymentID)}, "status": {status}}
	if note := strings.TrimSpace(request.Note); note != "" {
		form["note"] = []string{note}
	}
	if _, err := s.gateway.HandleWebhook(ctx, paymentGateway.WebhookRequest{Form: form}); err != nil {
		return PaymentResponse{}, err
	}
	return s.AdminGetPayment(ctx, request.PaymentID)
}

func (s *service) orderResponse(ctx context.Context, model ordersRepository.Model) (OrderResponse, error) {
	payments, err := s.orderPayments(ctx, model.ID)
	if err != nil {
		return OrderResponse{}, err
	}
	order := s.toOrder(ctx, model, payments, false, true)
	if order.Payment != nil {
		order.Payment.Action = s.currentAction(ctx, model, latest(payments))
	}
	return OrderResponse{Order: order}, nil
}

func (s *service) adminOrderResponse(ctx context.Context, model ordersRepository.Model) (OrderResponse, error) {
	byOrder, err := s.payments.ListPaymentsForOrders(ctx, []string{model.ID}, true)
	if err != nil {
		return OrderResponse{}, err
	}
	return OrderResponse{Order: s.toOrder(ctx, model, byOrder[model.ID], true, true)}, nil
}

func (s *service) listResponse(ctx context.Context, page ordersRepository.Page, admin bool) (ListOrdersResponse, error) {
	response := ListOrdersResponse{Orders: make([]Order, 0, len(page.Orders)), NextCursor: page.NextCursor}
	ids := make([]string, 0, len(page.Orders))
	for _, model := range page.Orders {
		ids = append(ids, model.ID)
	}
	byOrder, err := s.payments.ListPaymentsForOrders(ctx, ids, false)
	if err != nil {
		return ListOrdersResponse{}, err
	}
	for _, model := range page.Orders {
		response.Orders = append(response.Orders, s.toOrder(ctx, model, byOrder[model.ID], admin, false))
	}
	return response, nil
}
