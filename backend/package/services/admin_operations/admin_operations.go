package adminOperationsService

import (
	"context"
	"strings"
	"time"

	addressRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/address"
	cartsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/carts"
	imagesRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/images"
	ordersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/orders"
	paymentsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/payments"
	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
	usersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/users"
	addressesService "github.com/thesahibnanda-max/eCommerce/backend/package/services/addresses"
	googleAuthService "github.com/thesahibnanda-max/eCommerce/backend/package/services/google_auth"
	ordersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/orders"
	snapshotService "github.com/thesahibnanda-max/eCommerce/backend/package/services/snapshot"
)

type Service interface {
	Overview(ctx context.Context, request KeyRequest) (OverviewResponse, error)
	ListUsers(ctx context.Context, request PageRequest) (ListUsersResponse, error)
	GetUser(ctx context.Context, request IDRequest) (UserDetailResponse, error)
	ListCarts(ctx context.Context, request PageRequest) (ListCartsResponse, error)
	ListProducts(ctx context.Context, request PageRequest) (ListProductsResponse, error)
	ListImages(ctx context.Context, request PageRequest) (ListImagesResponse, error)
	ListOrders(ctx context.Context, request ListOrdersRequest) (ordersService.ListOrdersResponse, error)
	GetOrder(ctx context.Context, request IDRequest) (ordersService.OrderResponse, error)
	TransitionOrder(ctx context.Context, request TransitionOrderRequest) (ordersService.OrderResponse, error)
	SetTracking(ctx context.Context, request SetTrackingRequest) (ordersService.OrderResponse, error)
	ListPayments(ctx context.Context, request ListPaymentsRequest) (ordersService.ListPaymentsResponse, error)
	GetPayment(ctx context.Context, request IDRequest) (ordersService.PaymentResponse, error)
	GetPaymentProof(ctx context.Context, request IDRequest) (ProofImage, error)
	DecidePayment(ctx context.Context, request DecidePaymentRequest) (ordersService.PaymentResponse, error)
	SnapshotStatus(ctx context.Context, request KeyRequest) (SnapshotResponse, error)
	RunSnapshot(ctx context.Context, request KeyRequest) (SnapshotResponse, error)
	ConnectGoogle(ctx context.Context, request ConnectGoogleRequest) (ConnectGoogleResponse, error)
	GoogleCallback(ctx context.Context, request GoogleCallbackRequest) (string, error)
	DisconnectGoogle(ctx context.Context, request KeyRequest) (SnapshotResponse, error)
}

type Params struct {
	AdminKey         string
	Users            usersRepository.Repository
	Products         productsRepository.Repository
	Images           imagesRepository.Repository
	Carts            cartsRepository.Repository
	Addresses        addressRepository.Repository
	Orders           ordersRepository.Repository
	Payments         paymentsRepository.Repository
	OrdersService    ordersService.Service
	PaymentProvider  string
	Snapshots        snapshotService.Service
	Google           googleAuthService.Service
	SnapshotInterval time.Duration
	SnapshotTimeout  time.Duration
}

type service struct {
	adminKey         string
	users            usersRepository.Repository
	products         productsRepository.Repository
	images           imagesRepository.Repository
	carts            cartsRepository.Repository
	addresses        addressRepository.Repository
	orders           ordersRepository.Repository
	payments         paymentsRepository.Repository
	ordersSvc        ordersService.Service
	paymentProvider  string
	snapshots        snapshotService.Service
	google           googleAuthService.Service
	snapshotInterval time.Duration
	snapshotTimeout  time.Duration
}

func NewService(p Params) (Service, error) {
	adminKey := strings.TrimSpace(p.AdminKey)
	if adminKey == "" {
		return nil, ErrAdminKeyRequired
	}
	if (p.Snapshots == nil) != (p.Google == nil) || p.Snapshots != nil && (p.SnapshotInterval <= 0 || p.SnapshotTimeout <= 0) {
		return nil, ErrDependencyMissing
	}
	if p.Users == nil || p.Products == nil || p.Images == nil || p.Carts == nil || p.Addresses == nil || p.Orders == nil || p.Payments == nil || p.OrdersService == nil {
		return nil, ErrDependencyMissing
	}
	return &service{
		adminKey:         adminKey,
		users:            p.Users,
		products:         p.Products,
		images:           p.Images,
		carts:            p.Carts,
		addresses:        p.Addresses,
		orders:           p.Orders,
		payments:         p.Payments,
		ordersSvc:        p.OrdersService,
		paymentProvider:  p.PaymentProvider,
		snapshots:        p.Snapshots,
		google:           p.Google,
		snapshotInterval: p.SnapshotInterval,
		snapshotTimeout:  p.SnapshotTimeout,
	}, nil
}

func (s *service) Overview(ctx context.Context, request KeyRequest) (OverviewResponse, error) {
	if err := s.authorize(ctx, request.AdminKey); err != nil {
		return OverviewResponse{}, err
	}
	_, users, err := s.users.ListUsers(ctx, 0, 1)
	if err != nil {
		return OverviewResponse{}, err
	}
	_, products, err := s.products.ListProducts(ctx, 0, 1)
	if err != nil {
		return OverviewResponse{}, err
	}
	_, images, err := s.images.ListImages(ctx, 0, 1)
	if err != nil {
		return OverviewResponse{}, err
	}
	_, carts, err := s.carts.ListCarts(ctx, 0, 1)
	if err != nil {
		return OverviewResponse{}, err
	}
	orderCounts, err := s.orders.CountOrdersByState(ctx)
	if err != nil {
		return OverviewResponse{}, err
	}
	paymentCounts, err := s.payments.CountPaymentsByState(ctx)
	if err != nil {
		return OverviewResponse{}, err
	}
	overview := Overview{
		Users:           users,
		Products:        products,
		Images:          images,
		Carts:           carts,
		OrdersByState:   map[string]int64{},
		PaymentsByState: map[string]int64{},
		PaymentProvider: s.paymentProvider,
	}
	for state, count := range orderCounts {
		overview.OrdersByState[state.String()] = count
	}
	for state, count := range paymentCounts {
		overview.PaymentsByState[state.String()] = count
	}
	overview.PaymentsAwaitingReview = paymentCounts[paymentsRepository.StateVerifying]
	overview.OrdersToFulfil = orderCounts[ordersRepository.OrderStateApproved] + orderCounts[ordersRepository.OrderStateStartedShipping]
	return OverviewResponse{Overview: overview}, nil
}

func (s *service) ListUsers(ctx context.Context, request PageRequest) (ListUsersResponse, error) {
	if err := s.authorize(ctx, request.AdminKey); err != nil {
		return ListUsersResponse{}, err
	}
	models, total, err := s.users.ListUsers(ctx, request.Offset, pageLimit(request.Limit))
	if err != nil {
		return ListUsersResponse{}, err
	}
	users := make([]User, 0, len(models))
	for _, m := range models {
		users = append(users, toUser(m))
	}
	return ListUsersResponse{Users: users, Total: total}, nil
}

func (s *service) GetUser(ctx context.Context, request IDRequest) (UserDetailResponse, error) {
	if err := s.authorize(ctx, request.AdminKey); err != nil {
		return UserDetailResponse{}, err
	}
	id := strings.TrimSpace(request.ID)
	model, err := s.users.GetUserByID(ctx, id)
	if err != nil {
		return UserDetailResponse{}, err
	}
	addresses, err := s.addresses.ListAddresses(ctx, id)
	if err != nil {
		return UserDetailResponse{}, err
	}
	rawCart, err := s.carts.GetCart(ctx, id)
	if err != nil {
		return UserDetailResponse{}, err
	}
	cart, err := s.toCart(ctx, rawCart)
	if err != nil {
		return UserDetailResponse{}, err
	}
	orders, err := s.ordersSvc.AdminListUserOrders(ctx, id)
	if err != nil {
		return UserDetailResponse{}, err
	}
	return UserDetailResponse{User: toUser(model), Addresses: addressesService.ToAddresses(addresses), Cart: cart, Orders: orders.Orders}, nil
}

func (s *service) ListCarts(ctx context.Context, request PageRequest) (ListCartsResponse, error) {
	if err := s.authorize(ctx, request.AdminKey); err != nil {
		return ListCartsResponse{}, err
	}
	models, total, err := s.carts.ListCarts(ctx, request.Offset, pageLimit(request.Limit))
	if err != nil {
		return ListCartsResponse{}, err
	}
	carts := make([]Cart, 0, len(models))
	for _, m := range models {
		cart, err := s.toCart(ctx, m)
		if err != nil {
			return ListCartsResponse{}, err
		}
		carts = append(carts, cart)
	}
	return ListCartsResponse{Carts: carts, Total: total}, nil
}

func (s *service) ListProducts(ctx context.Context, request PageRequest) (ListProductsResponse, error) {
	if err := s.authorize(ctx, request.AdminKey); err != nil {
		return ListProductsResponse{}, err
	}
	models, total, err := s.products.ListProducts(ctx, request.Offset, pageLimit(request.Limit))
	if err != nil {
		return ListProductsResponse{}, err
	}
	products := make([]Product, 0, len(models))
	for _, m := range models {
		products = append(products, toProduct(m))
	}
	return ListProductsResponse{Products: products, Total: total}, nil
}

func (s *service) ListImages(ctx context.Context, request PageRequest) (ListImagesResponse, error) {
	if err := s.authorize(ctx, request.AdminKey); err != nil {
		return ListImagesResponse{}, err
	}
	models, total, err := s.images.ListImages(ctx, request.Offset, pageLimit(request.Limit))
	if err != nil {
		return ListImagesResponse{}, err
	}
	images := make([]Image, 0, len(models))
	for _, m := range models {
		images = append(images, toImage(m))
	}
	return ListImagesResponse{Images: images, Total: total}, nil
}

func (s *service) ListOrders(ctx context.Context, request ListOrdersRequest) (ordersService.ListOrdersResponse, error) {
	if err := s.authorize(ctx, request.AdminKey); err != nil {
		return ordersService.ListOrdersResponse{}, err
	}
	return s.ordersSvc.AdminListOrders(ctx, ordersService.AdminListOrdersRequest{States: request.States, Cursor: request.Cursor, Limit: request.Limit})
}

func (s *service) GetOrder(ctx context.Context, request IDRequest) (ordersService.OrderResponse, error) {
	if err := s.authorize(ctx, request.AdminKey); err != nil {
		return ordersService.OrderResponse{}, err
	}
	return s.ordersSvc.AdminGetOrder(ctx, request.ID)
}

func (s *service) TransitionOrder(ctx context.Context, request TransitionOrderRequest) (ordersService.OrderResponse, error) {
	if err := s.authorize(ctx, request.AdminKey); err != nil {
		return ordersService.OrderResponse{}, err
	}
	return s.ordersSvc.AdminTransitionOrder(ctx, ordersService.AdminTransitionOrderRequest{OrderID: request.ID, To: request.To, Reason: request.Reason})
}

func (s *service) SetTracking(ctx context.Context, request SetTrackingRequest) (ordersService.OrderResponse, error) {
	if err := s.authorize(ctx, request.AdminKey); err != nil {
		return ordersService.OrderResponse{}, err
	}
	return s.ordersSvc.AdminSetTracking(ctx, ordersService.AdminSetTrackingRequest{OrderID: request.ID, Courier: request.Courier, Number: request.Number, URL: request.URL})
}

func (s *service) ListPayments(ctx context.Context, request ListPaymentsRequest) (ordersService.ListPaymentsResponse, error) {
	if err := s.authorize(ctx, request.AdminKey); err != nil {
		return ordersService.ListPaymentsResponse{}, err
	}
	return s.ordersSvc.AdminListPayments(ctx, ordersService.AdminListPaymentsRequest{States: request.States, Provider: request.Provider, Cursor: request.Cursor, Limit: request.Limit})
}

func (s *service) GetPayment(ctx context.Context, request IDRequest) (ordersService.PaymentResponse, error) {
	if err := s.authorize(ctx, request.AdminKey); err != nil {
		return ordersService.PaymentResponse{}, err
	}
	return s.ordersSvc.AdminGetPayment(ctx, request.ID)
}

func (s *service) GetPaymentProof(ctx context.Context, request IDRequest) (ProofImage, error) {
	if err := s.authorize(ctx, request.AdminKey); err != nil {
		return ProofImage{}, err
	}
	content, contentType, err := s.payments.GetProofImage(ctx, strings.TrimSpace(request.ID))
	if err != nil {
		return ProofImage{}, err
	}
	return ProofImage{Content: content, ContentType: contentType}, nil
}

func (s *service) DecidePayment(ctx context.Context, request DecidePaymentRequest) (ordersService.PaymentResponse, error) {
	if err := s.authorize(ctx, request.AdminKey); err != nil {
		return ordersService.PaymentResponse{}, err
	}
	return s.ordersSvc.AdminDecidePayment(ctx, ordersService.AdminDecidePaymentRequest{PaymentID: request.ID, Status: request.Status, Note: request.Note})
}
