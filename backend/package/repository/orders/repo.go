package ordersRepository

//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	ordersQueries "github.com/thesahibnanda-max/eCommerce/backend/package/repository/orders/queries"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

type Repository interface {
	CreateOrder(ctx context.Context, input CreateOrderInput) (Model, error)
	GetOrder(ctx context.Context, orderID string) (Model, error)
	GetUserOrder(ctx context.Context, userID, orderID string) (Model, error)
	GetUserOrderByIdempotencyKey(ctx context.Context, userID, idempotencyKey string) (Model, error)
	ListUserOrders(ctx context.Context, userID string, query ListQuery) (Page, error)
	ListOrders(ctx context.Context, query ListQuery) (Page, error)
	TransitionOrder(ctx context.Context, orderID string, to OrderState, actor Actor, reason string) (Model, error)
	SetPaymentGateway(ctx context.Context, orderID, gateway string) (Model, error)
	RecordPaymentEvent(ctx context.Context, orderID string, event PaymentEvent) (Model, error)
	UpdateMetadata(ctx context.Context, orderID string, mutate func(map[string]any) (map[string]any, error)) (Model, error)
	CountOrdersByState(ctx context.Context) (map[OrderState]int64, error)
	Close() error
}

type repository struct {
	writer   *sql.DB
	reader   *sql.DB
	now      func() (time.Time, error)
	location *time.Location
}

func NewRepository(path []string, permission os.FileMode) (Repository, error) {
	file := filepath.Join(path...)
	now, err := utils.TimeNow()
	if err != nil {
		return nil, err
	}
	handle, err := os.OpenFile(file, os.O_CREATE|os.O_RDWR, permission)
	if err != nil {
		return nil, err
	}
	if err := handle.Close(); err != nil {
		return nil, err
	}
	writer, err := sql.Open("sqlite", dsn(file, true))
	if err != nil {
		return nil, err
	}
	writer.SetMaxOpenConns(1)
	if _, err := writer.Exec(schemaSQL); err != nil {
		_ = writer.Close()
		return nil, err
	}
	reader, err := sql.Open("sqlite", dsn(file, false))
	if err != nil {
		_ = writer.Close()
		return nil, err
	}
	reader.SetMaxOpenConns(readerConnections)
	if err := reader.Ping(); err != nil {
		_ = writer.Close()
		_ = reader.Close()
		return nil, err
	}
	return &repository{writer: writer, reader: reader, now: utils.TimeNow, location: now.Location()}, nil
}

func (r *repository) read(ctx context.Context, fn func(q *ordersQueries.Queries) error) error {
	return withTx(ctx, r.reader, &sql.TxOptions{ReadOnly: true}, fn)
}

func (r *repository) write(ctx context.Context, fn func(q *ordersQueries.Queries) error) error {
	return withTx(ctx, r.writer, nil, fn)
}

func (r *repository) stamp() (int64, error) {
	now, err := r.now()
	if err != nil {
		return 0, err
	}
	return now.UnixMicro(), nil
}

func (r *repository) CreateOrder(ctx context.Context, input CreateOrderInput) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	normalized, subtotal, total, err := normalizeInput(input)
	if err != nil {
		return Model{}, err
	}
	hash, err := requestHash(normalized)
	if err != nil {
		return Model{}, err
	}
	metadata, err := encodeMetadata(normalized.Metadata)
	if err != nil {
		return Model{}, err
	}
	orderID, err := newOrderID()
	if err != nil {
		return Model{}, err
	}
	at, err := r.stamp()
	if err != nil {
		return Model{}, err
	}
	var order Model
	err = r.write(ctx, func(q *ordersQueries.Queries) error {
		existing, err := q.GetOrderByIdempotencyKey(ctx, ordersQueries.GetOrderByIdempotencyKeyParams{
			UserID:         normalized.UserID,
			IdempotencyKey: normalized.IdempotencyKey,
		})
		switch {
		case err == nil:
			if existing.RequestHash != hash {
				return ErrIdempotencyKeyConflict
			}
			order, err = r.loadOrder(ctx, q, existing.ID)
			return err
		case !errors.Is(err, sql.ErrNoRows):
			return err
		}
		address := normalized.ShippingAddress
		if err := q.InsertOrder(ctx, ordersQueries.InsertOrderParams{
			ID:             orderID,
			UserID:         normalized.UserID,
			IdempotencyKey: normalized.IdempotencyKey,
			RequestHash:    hash,
			State:          OrderStateCreated.String(),
			Currency:       normalized.Currency.String(),
			Subtotal:       subtotal,
			ShippingFee:    normalized.ShippingFee,
			Discount:       normalized.Discount,
			TotalPayable:   total,
			AddressID:      nullString(address.AddressID),
			ShipFullName:   address.FullName,
			ShipPhone:      address.PhoneNumber,
			ShipLine1:      address.AddressLine1,
			ShipLine2:      address.AddressLine2,
			ShipLandmark:   address.Landmark,
			ShipCity:       address.City,
			ShipState:      address.State,
			ShipStateCode:  address.StateCode,
			ShipPincode:    address.Pincode,
			ShipCountry:    address.Country,
			Metadata:       metadata,
			CreatedAt:      at,
			UpdatedAt:      at,
		}); err != nil {
			return err
		}
		for position, item := range normalized.Items {
			attributes, err := utils.FromObjectToStringJSON(item.Attributes)
			if err != nil {
				return err
			}
			if err := q.InsertOrderItem(ctx, ordersQueries.InsertOrderItemParams{
				OrderID:     orderID,
				Position:    int64(position),
				ProductID:   item.ProductID,
				VariantID:   item.ProductVariantID,
				Sku:         item.SKU,
				ProductName: item.ProductName,
				Attributes:  attributes,
				UnitPrice:   item.UnitPrice,
				Quantity:    item.Quantity,
				LineTotal:   item.LineTotal,
			}); err != nil {
				return err
			}
		}
		if err := q.InsertStateChange(ctx, ordersQueries.InsertStateChangeParams{
			OrderID:   orderID,
			ToState:   OrderStateCreated.String(),
			Reason:    initialHistoryReason,
			Actor:     ActorUser.String(),
			CreatedAt: at,
		}); err != nil {
			return err
		}
		order, err = r.loadOrder(ctx, q, orderID)
		return err
	})
	if err != nil {
		return Model{}, err
	}
	return order, nil
}

func (r *repository) GetOrder(ctx context.Context, orderID string) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	if err := validateOrderID(orderID); err != nil {
		return Model{}, err
	}
	var order Model
	err := r.read(ctx, func(q *ordersQueries.Queries) error {
		var err error
		order, err = r.loadOrder(ctx, q, orderID)
		return err
	})
	if err != nil {
		return Model{}, err
	}
	return order, nil
}

func (r *repository) GetUserOrder(ctx context.Context, userID, orderID string) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	if !utils.ValidateULID(userID) {
		return Model{}, ErrUserIDInvalid
	}
	order, err := r.GetOrder(ctx, orderID)
	if err != nil {
		return Model{}, err
	}
	if order.UserID != userID {
		return Model{}, ErrOrderNotFound
	}
	return order, nil
}

func (r *repository) GetUserOrderByIdempotencyKey(ctx context.Context, userID, idempotencyKey string) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	if !utils.ValidateULID(userID) {
		return Model{}, ErrUserIDInvalid
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" || utf8.RuneCountInString(idempotencyKey) > maxIdempotencyKey {
		return Model{}, ErrIdempotencyKeyInvalid
	}
	var model Model
	err := r.read(ctx, func(q *ordersQueries.Queries) error {
		row, err := q.GetOrderByIdempotencyKey(ctx, ordersQueries.GetOrderByIdempotencyKeyParams{UserID: userID, IdempotencyKey: idempotencyKey})
		if errors.Is(err, sql.ErrNoRows) {
			return ErrOrderNotFound
		}
		if err != nil {
			return err
		}
		model, err = r.loadOrder(ctx, q, row.ID)
		return err
	})
	return model, err
}

func (r *repository) ListUserOrders(ctx context.Context, userID string, query ListQuery) (Page, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Page{}, err
	}
	if !utils.ValidateULID(userID) {
		return Page{}, ErrUserIDInvalid
	}
	query, states, err := normalizeListQuery(query)
	if err != nil {
		return Page{}, err
	}
	cursorAt, cursorID, err := decodeCursor(query.Cursor)
	if err != nil {
		return Page{}, err
	}
	var page Page
	err = r.read(ctx, func(q *ordersQueries.Queries) error {
		rows, err := q.ListUserOrders(ctx, ordersQueries.ListUserOrdersParams{
			UserID:   userID,
			States:   states,
			CursorAt: cursorAt,
			CursorID: cursorID,
			Limit:    int64(query.Limit + 1),
		})
		if err != nil {
			return err
		}
		page, err = r.toPage(ctx, q, rows, query.Limit)
		return err
	})
	if err != nil {
		return Page{}, err
	}
	return page, nil
}

func (r *repository) ListOrders(ctx context.Context, query ListQuery) (Page, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Page{}, err
	}
	query, states, err := normalizeListQuery(query)
	if err != nil {
		return Page{}, err
	}
	cursorAt, cursorID, err := decodeCursor(query.Cursor)
	if err != nil {
		return Page{}, err
	}
	var page Page
	err = r.read(ctx, func(q *ordersQueries.Queries) error {
		rows, err := q.ListOrders(ctx, ordersQueries.ListOrdersParams{
			States:   states,
			CursorAt: cursorAt,
			CursorID: cursorID,
			Limit:    int64(query.Limit + 1),
		})
		if err != nil {
			return err
		}
		page, err = r.toPage(ctx, q, rows, query.Limit)
		return err
	})
	if err != nil {
		return Page{}, err
	}
	return page, nil
}

func (r *repository) TransitionOrder(ctx context.Context, orderID string, to OrderState, actor Actor, reason string) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	if err := validateOrderID(orderID); err != nil {
		return Model{}, err
	}
	if !to.IsValid() {
		return Model{}, ErrStateInvalid
	}
	if !actor.IsValid() {
		return Model{}, ErrActorInvalid
	}
	reason = cleanString(reason)
	if !utf8.ValidString(reason) || utf8.RuneCountInString(reason) > maxReasonRunes {
		return Model{}, ErrReasonTooLong
	}
	at, err := r.stamp()
	if err != nil {
		return Model{}, err
	}
	var order Model
	err = r.write(ctx, func(q *ordersQueries.Queries) error {
		current, err := r.loadOrder(ctx, q, orderID)
		if err != nil {
			return err
		}
		from := current.OrderState.CurrentState().Value()
		if !current.OrderState.TransitionTo(stateOf(to)) {
			return fmt.Errorf("%w: %s -> %s", ErrTransitionNotAllowed, from, to)
		}
		affected, err := q.UpdateOrderState(ctx, ordersQueries.UpdateOrderStateParams{
			ToState:   to.String(),
			UpdatedAt: at,
			ID:        orderID,
			FromState: from.String(),
		})
		if err != nil {
			return err
		}
		if affected != 1 {
			return ErrConcurrentUpdate
		}
		if err := q.InsertStateChange(ctx, ordersQueries.InsertStateChangeParams{
			OrderID:   orderID,
			FromState: nullString(from.String()),
			ToState:   to.String(),
			Reason:    reason,
			Actor:     actor.String(),
			CreatedAt: at,
		}); err != nil {
			return err
		}
		order, err = r.loadOrder(ctx, q, orderID)
		return err
	})
	if err != nil {
		return Model{}, err
	}
	return order, nil
}

func (r *repository) SetPaymentGateway(ctx context.Context, orderID, gateway string) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	if err := validateOrderID(orderID); err != nil {
		return Model{}, err
	}
	gateway = strings.TrimSpace(gateway)
	if gateway == "" || !utf8.ValidString(gateway) || utf8.RuneCountInString(gateway) > maxGatewayRunes {
		return Model{}, ErrPaymentGatewayInvalid
	}
	at, err := r.stamp()
	if err != nil {
		return Model{}, err
	}
	var order Model
	err = r.write(ctx, func(q *ordersQueries.Queries) error {
		current, err := r.loadOrder(ctx, q, orderID)
		if err != nil {
			return err
		}
		state := current.OrderState.CurrentState().Value()
		if state != OrderStateCreated && state != OrderStateProcessing {
			return ErrPaymentGatewayLocked
		}
		if _, err := q.UpdatePaymentGateway(ctx, ordersQueries.UpdatePaymentGatewayParams{
			PaymentGateway: nullString(gateway),
			UpdatedAt:      at,
			ID:             orderID,
		}); err != nil {
			return err
		}
		order, err = r.loadOrder(ctx, q, orderID)
		return err
	})
	if err != nil {
		return Model{}, err
	}
	return order, nil
}

func (r *repository) RecordPaymentEvent(ctx context.Context, orderID string, event PaymentEvent) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	if err := validateOrderID(orderID); err != nil {
		return Model{}, err
	}
	event.Gateway = strings.TrimSpace(event.Gateway)
	event.GatewayEventID = strings.TrimSpace(event.GatewayEventID)
	event.EventType = strings.TrimSpace(event.EventType)
	event.GatewayOrderID = strings.TrimSpace(event.GatewayOrderID)
	event.GatewayPaymentID = strings.TrimSpace(event.GatewayPaymentID)
	event.Status = strings.TrimSpace(event.Status)
	if event.Gateway == "" || event.GatewayEventID == "" || event.EventType == "" || event.Amount < 0 {
		return Model{}, ErrPaymentEventInvalid
	}
	if event.Raw == nil {
		event.Raw = map[string]any{}
	}
	raw, err := encodeJSON(event.Raw, maxRawPaymentBytes, ErrMetadataTooLarge)
	if err != nil {
		return Model{}, err
	}
	at, err := r.stamp()
	if err != nil {
		return Model{}, err
	}
	var order Model
	err = r.write(ctx, func(q *ordersQueries.Queries) error {
		if _, err := q.GetOrder(ctx, orderID); errors.Is(err, sql.ErrNoRows) {
			return ErrOrderNotFound
		} else if err != nil {
			return err
		}
		count, err := q.PaymentEventExists(ctx, ordersQueries.PaymentEventExistsParams{
			Gateway:        event.Gateway,
			GatewayEventID: event.GatewayEventID,
		})
		if err != nil {
			return err
		}
		if count > 0 {
			return ErrPaymentEventDuplicate
		}
		if err := q.InsertPaymentEvent(ctx, ordersQueries.InsertPaymentEventParams{
			OrderID:          orderID,
			Gateway:          event.Gateway,
			GatewayEventID:   event.GatewayEventID,
			EventType:        event.EventType,
			GatewayOrderID:   event.GatewayOrderID,
			GatewayPaymentID: event.GatewayPaymentID,
			Status:           event.Status,
			Amount:           event.Amount,
			Raw:              raw,
			CreatedAt:        at,
		}); err != nil {
			return err
		}
		order, err = r.loadOrder(ctx, q, orderID)
		return err
	})
	if err != nil {
		return Model{}, err
	}
	return order, nil
}

func (r *repository) UpdateMetadata(ctx context.Context, orderID string, mutate func(map[string]any) (map[string]any, error)) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	if err := validateOrderID(orderID); err != nil {
		return Model{}, err
	}
	if mutate == nil {
		return Model{}, ErrMutateRequired
	}
	at, err := r.stamp()
	if err != nil {
		return Model{}, err
	}
	var order Model
	err = r.write(ctx, func(q *ordersQueries.Queries) error {
		current, err := r.loadOrder(ctx, q, orderID)
		if err != nil {
			return err
		}
		next, err := runMutate(mutate, current.Metadata)
		if err != nil {
			return err
		}
		metadata, err := encodeMetadata(next)
		if err != nil {
			return err
		}
		if _, err := q.UpdateMetadata(ctx, ordersQueries.UpdateMetadataParams{
			Metadata:  metadata,
			UpdatedAt: at,
			ID:        orderID,
		}); err != nil {
			return err
		}
		order, err = r.loadOrder(ctx, q, orderID)
		return err
	})
	if err != nil {
		return Model{}, err
	}
	return order, nil
}

func (r *repository) CountOrdersByState(ctx context.Context) (map[OrderState]int64, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return nil, err
	}
	counts := make(map[OrderState]int64, len(orderStates))
	for _, state := range orderStates {
		counts[state] = 0
	}
	err := r.read(ctx, func(q *ordersQueries.Queries) error {
		rows, err := q.CountOrdersByState(ctx)
		if err != nil {
			return err
		}
		for _, row := range rows {
			state, err := ParseOrderState(row.State)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrStoredDataCorrupted, err)
			}
			counts[state] = row.Total
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return counts, nil
}

func (r *repository) Close() error {
	return errors.Join(r.reader.Close(), r.writer.Close())
}
