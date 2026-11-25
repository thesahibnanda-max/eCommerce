package ordersRepository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	ordersQueries "github.com/thesahibnanda-max/eCommerce/backend/package/repository/orders/queries"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	stateMachineUtils "github.com/thesahibnanda-max/eCommerce/backend/package/utils/state_machine"
)

const (
	ulidLength           = 26
	orderIDLength        = 63
	maxItems             = 100
	maxIdempotencyKey    = 128
	maxReasonRunes       = 500
	maxGatewayRunes      = 64
	maxMetadataBytes     = 8192
	maxRawPaymentBytes   = 65536
	defaultLimit         = 20
	maxLimit             = 100
	busyTimeoutMillis    = 5000
	readerConnections    = 8
	initialHistoryReason = "order placed"
)

var pincodePattern = regexp.MustCompile(`^[1-9][0-9]{5}$`)

func newOrderID() (string, error) {
	suffix, err := utils.NewUUIDv7()
	if err != nil {
		return "", err
	}
	return utils.NewULID() + "-" + suffix, nil
}

func validateOrderID(id string) error {
	if len(id) != orderIDLength || id[ulidLength] != '-' || !utils.ValidateULID(id[:ulidLength]) || !utils.ValidateUUIDv7(id[ulidLength+1:]) {
		return ErrOrderIDInvalid
	}
	return nil
}

func dsn(path string, writer bool) string {
	params := url.Values{}
	params.Add("_pragma", "foreign_keys(1)")
	params.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busyTimeoutMillis))
	params.Add("_pragma", "journal_mode(WAL)")
	params.Add("_pragma", "synchronous(NORMAL)")
	params.Add("_pragma", "cache_size(-8000)")
	params.Add("_pragma", "temp_store(MEMORY)")
	if writer {
		params.Add("_txlock", "immediate")
	} else {
		params.Add("_pragma", "query_only(1)")
	}
	return "file:" + path + "?" + params.Encode()
}

func cleanString(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func checkedMul(a, b int64) (int64, error) {
	if a != 0 && b > math.MaxInt64/a {
		return 0, ErrAmountOverflow
	}
	return a * b, nil
}

func checkedAdd(a, b int64) (int64, error) {
	if b > math.MaxInt64-a {
		return 0, ErrAmountOverflow
	}
	return a + b, nil
}

func normalizeAddress(address ShippingAddress) (ShippingAddress, error) {
	address = ShippingAddress{
		AddressID:    strings.TrimSpace(address.AddressID),
		FullName:     cleanString(address.FullName),
		PhoneNumber:  cleanString(address.PhoneNumber),
		AddressLine1: cleanString(address.AddressLine1),
		AddressLine2: cleanString(address.AddressLine2),
		Landmark:     cleanString(address.Landmark),
		City:         cleanString(address.City),
		State:        cleanString(address.State),
		StateCode:    cleanString(address.StateCode),
		Pincode:      strings.ReplaceAll(strings.TrimSpace(address.Pincode), " ", ""),
		Country:      cleanString(address.Country),
	}
	if address.AddressID != "" && !utils.ValidateULID(address.AddressID) {
		return ShippingAddress{}, ErrAddressIncomplete
	}
	if address.FullName == "" || address.PhoneNumber == "" || address.AddressLine1 == "" || address.City == "" ||
		address.State == "" || address.StateCode == "" || address.Country == "" || !pincodePattern.MatchString(address.Pincode) {
		return ShippingAddress{}, ErrAddressIncomplete
	}
	return address, nil
}

func normalizeItems(items []Item) ([]Item, int64, error) {
	if len(items) == 0 {
		return nil, 0, ErrItemsRequired
	}
	if len(items) > maxItems {
		return nil, 0, ErrTooManyItems
	}
	normalized := make([]Item, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	var subtotal int64
	for _, item := range items {
		item.ProductID = strings.TrimSpace(item.ProductID)
		item.ProductVariantID = strings.TrimSpace(item.ProductVariantID)
		item.SKU = cleanString(item.SKU)
		item.ProductName = cleanString(item.ProductName)
		if !utils.ValidateULID(item.ProductID) || !utils.ValidateULID(item.ProductVariantID) || item.SKU == "" || item.ProductName == "" {
			return nil, 0, ErrItemInvalid
		}
		if _, dup := seen[item.ProductVariantID]; dup {
			return nil, 0, ErrDuplicateVariant
		}
		seen[item.ProductVariantID] = struct{}{}
		if item.Quantity < 1 {
			return nil, 0, ErrQuantityInvalid
		}
		if item.UnitPrice < 1 {
			return nil, 0, ErrPriceInvalid
		}
		lineTotal, err := checkedMul(item.UnitPrice, item.Quantity)
		if err != nil {
			return nil, 0, err
		}
		if subtotal, err = checkedAdd(subtotal, lineTotal); err != nil {
			return nil, 0, err
		}
		item.LineTotal = lineTotal
		attributes := make(map[string]string, len(item.Attributes))
		for key, value := range item.Attributes {
			attributes[key] = value
		}
		item.Attributes = attributes
		normalized = append(normalized, item)
	}
	return normalized, subtotal, nil
}

func normalizeInput(input CreateOrderInput) (CreateOrderInput, int64, int64, error) {
	if !utils.ValidateULID(input.UserID) {
		return CreateOrderInput{}, 0, 0, ErrUserIDInvalid
	}
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.IdempotencyKey == "" || len(input.IdempotencyKey) > maxIdempotencyKey || !utf8.ValidString(input.IdempotencyKey) {
		return CreateOrderInput{}, 0, 0, ErrIdempotencyKeyInvalid
	}
	if input.Currency == "" {
		input.Currency = IndianNationalRupeeCurrency
	}
	if !input.Currency.IsValid() {
		return CreateOrderInput{}, 0, 0, ErrCurrencyInvalid
	}
	address, err := normalizeAddress(input.ShippingAddress)
	if err != nil {
		return CreateOrderInput{}, 0, 0, err
	}
	input.ShippingAddress = address
	items, subtotal, err := normalizeItems(input.Items)
	if err != nil {
		return CreateOrderInput{}, 0, 0, err
	}
	input.Items = items
	if input.ShippingFee < 0 || input.Discount < 0 {
		return CreateOrderInput{}, 0, 0, ErrAmountInvalid
	}
	gross, err := checkedAdd(subtotal, input.ShippingFee)
	if err != nil {
		return CreateOrderInput{}, 0, 0, err
	}
	if input.Discount > gross {
		return CreateOrderInput{}, 0, 0, ErrAmountInvalid
	}
	return input, subtotal, gross - input.Discount, nil
}

func requestHash(input CreateOrderInput) (string, error) {
	type hashedItem struct {
		ProductID        string
		ProductVariantID string
		SKU              string
		ProductName      string
		Attributes       [][2]string
		UnitPrice        int64
		Quantity         int64
	}
	items := make([]hashedItem, 0, len(input.Items))
	for _, item := range input.Items {
		attributes := make([][2]string, 0, len(item.Attributes))
		for key, value := range item.Attributes {
			attributes = append(attributes, [2]string{key, value})
		}
		sort.Slice(attributes, func(i, j int) bool { return attributes[i][0] < attributes[j][0] })
		items = append(items, hashedItem{
			ProductID:        item.ProductID,
			ProductVariantID: item.ProductVariantID,
			SKU:              item.SKU,
			ProductName:      item.ProductName,
			Attributes:       attributes,
			UnitPrice:        item.UnitPrice,
			Quantity:         item.Quantity,
		})
	}
	raw, err := utils.FromObjectToStringJSON(struct {
		Currency        Currency
		ShippingFee     int64
		Discount        int64
		ShippingAddress ShippingAddress
		Items           []hashedItem
	}{input.Currency, input.ShippingFee, input.Discount, input.ShippingAddress, items})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:]), nil
}

func encodeJSON[T any](value T, limit int, tooLarge error) (string, error) {
	raw, err := utils.FromObjectToStringJSON(value)
	if err != nil {
		return "", err
	}
	if len(raw) > limit {
		return "", tooLarge
	}
	return raw, nil
}

func encodeMetadata(metadata map[string]any) (string, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	return encodeJSON(metadata, maxMetadataBytes, ErrMetadataTooLarge)
}

func decodeMap[T any](raw string) (map[string]T, error) {
	value, err := utils.FromStringToObjectJSON[map[string]T](raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrStoredDataCorrupted, err)
	}
	if value == nil {
		value = map[string]T{}
	}
	return value, nil
}

func encodeCursor(order ordersQueries.Order) (string, error) {
	raw, err := utils.FromObjectToStringJSON(cursor{CreatedAt: order.CreatedAt, ID: order.ID})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString([]byte(raw)), nil
}

func decodeCursor(value string) (int64, string, error) {
	if value == "" {
		return math.MaxInt64, "", nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return 0, "", ErrCursorInvalid
	}
	decoded, err := utils.FromStringToObjectJSON[cursor](string(raw))
	if err != nil || validateOrderID(decoded.ID) != nil {
		return 0, "", ErrCursorInvalid
	}
	return decoded.CreatedAt, decoded.ID, nil
}

func normalizeListQuery(query ListQuery) (ListQuery, string, error) {
	if query.Limit == 0 {
		query.Limit = defaultLimit
	}
	if query.Limit < 1 || query.Limit > maxLimit {
		return ListQuery{}, "", ErrLimitInvalid
	}
	states := make([]string, 0, len(query.States))
	for _, state := range query.States {
		if !state.IsValid() {
			return ListQuery{}, "", ErrStateInvalid
		}
		states = append(states, state.String())
	}
	statesJSON, err := utils.FromObjectToStringJSON(states)
	if err != nil {
		return ListQuery{}, "", err
	}
	return query, statesJSON, nil
}

func (r *repository) toTime(micros int64) time.Time {
	return time.UnixMicro(micros).In(r.location)
}

func (r *repository) toModel(row ordersQueries.Order, items []ordersQueries.OrderItem, history []ordersQueries.OrderStateHistory, events []ordersQueries.OrderPaymentEvent) (Model, error) {
	state, err := ParseOrderState(row.State)
	if err != nil {
		return Model{}, fmt.Errorf("%w: %w", ErrStoredDataCorrupted, err)
	}
	machine, err := NewOrderStateMachine(state)
	if err != nil {
		return Model{}, err
	}
	metadata, err := decodeMap[any](row.Metadata)
	if err != nil {
		return Model{}, err
	}
	model := Model{
		ID:             row.ID,
		UserID:         row.UserID,
		IdempotencyKey: row.IdempotencyKey,
		OrderState:     machine,
		Currency:       Currency(row.Currency),
		Subtotal:       row.Subtotal,
		ShippingFee:    row.ShippingFee,
		Discount:       row.Discount,
		TotalPayable:   row.TotalPayable,
		ShippingAddress: ShippingAddress{
			AddressID:    row.AddressID.String,
			FullName:     row.ShipFullName,
			PhoneNumber:  row.ShipPhone,
			AddressLine1: row.ShipLine1,
			AddressLine2: row.ShipLine2,
			Landmark:     row.ShipLandmark,
			City:         row.ShipCity,
			State:        row.ShipState,
			StateCode:    row.ShipStateCode,
			Pincode:      row.ShipPincode,
			Country:      row.ShipCountry,
		},
		PaymentGateway: row.PaymentGateway.String,
		Items:          make([]Item, 0, len(items)),
		StateHistory:   make([]StateChange, 0, len(history)),
		PaymentEvents:  make([]PaymentEvent, 0, len(events)),
		Metadata:       metadata,
		CreatedAt:      r.toTime(row.CreatedAt),
		UpdatedAt:      r.toTime(row.UpdatedAt),
	}
	for _, item := range items {
		attributes, err := decodeMap[string](item.Attributes)
		if err != nil {
			return Model{}, err
		}
		model.Items = append(model.Items, Item{
			ProductID:        item.ProductID,
			ProductVariantID: item.VariantID,
			SKU:              item.Sku,
			ProductName:      item.ProductName,
			Attributes:       attributes,
			UnitPrice:        item.UnitPrice,
			Quantity:         item.Quantity,
			LineTotal:        item.LineTotal,
		})
	}
	for _, change := range history {
		model.StateHistory = append(model.StateHistory, StateChange{
			FromState: OrderState(change.FromState.String),
			ToState:   OrderState(change.ToState),
			Reason:    change.Reason,
			Actor:     Actor(change.Actor),
			CreatedAt: r.toTime(change.CreatedAt),
		})
	}
	for _, event := range events {
		raw, err := decodeMap[any](event.Raw)
		if err != nil {
			return Model{}, err
		}
		model.PaymentEvents = append(model.PaymentEvents, PaymentEvent{
			Gateway:          event.Gateway,
			GatewayEventID:   event.GatewayEventID,
			EventType:        event.EventType,
			GatewayOrderID:   event.GatewayOrderID,
			GatewayPaymentID: event.GatewayPaymentID,
			Status:           event.Status,
			Amount:           event.Amount,
			Raw:              raw,
			CreatedAt:        r.toTime(event.CreatedAt),
		})
	}
	return model, nil
}

func (r *repository) loadOrder(ctx context.Context, q *ordersQueries.Queries, orderID string) (Model, error) {
	row, err := q.GetOrder(ctx, orderID)
	if errors.Is(err, sql.ErrNoRows) {
		return Model{}, ErrOrderNotFound
	}
	if err != nil {
		return Model{}, err
	}
	items, err := q.ListItemsByOrderIDs(ctx, []string{orderID})
	if err != nil {
		return Model{}, err
	}
	history, err := q.ListStateHistory(ctx, orderID)
	if err != nil {
		return Model{}, err
	}
	events, err := q.ListPaymentEvents(ctx, orderID)
	if err != nil {
		return Model{}, err
	}
	return r.toModel(row, items, history, events)
}

func (r *repository) toPage(ctx context.Context, q *ordersQueries.Queries, rows []ordersQueries.Order, limit int) (Page, error) {
	page := Page{Orders: make([]Model, 0, min(len(rows), limit))}
	if len(rows) > limit {
		rows = rows[:limit]
		next, err := encodeCursor(rows[len(rows)-1])
		if err != nil {
			return Page{}, err
		}
		page.NextCursor = next
	}
	if len(rows) == 0 {
		return page, nil
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	items, err := q.ListItemsByOrderIDs(ctx, ids)
	if err != nil {
		return Page{}, err
	}
	byOrder := make(map[string][]ordersQueries.OrderItem, len(rows))
	for _, item := range items {
		byOrder[item.OrderID] = append(byOrder[item.OrderID], item)
	}
	for _, row := range rows {
		model, err := r.toModel(row, byOrder[row.ID], nil, nil)
		if err != nil {
			return Page{}, err
		}
		page.Orders = append(page.Orders, model)
	}
	return page, nil
}

func withTx(ctx context.Context, db *sql.DB, opts *sql.TxOptions, fn func(q *ordersQueries.Queries) error) (err error) {
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = tx.Rollback()
			err = fmt.Errorf("%w: %v", ErrTransactionPanicked, recovered)
		}
	}()
	if err = fn(ordersQueries.New(tx)); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func runMutate(mutate func(map[string]any) (map[string]any, error), current map[string]any) (next map[string]any, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			next = nil
			err = fmt.Errorf("%w: %v", ErrMutatePanicked, recovered)
		}
	}()
	return mutate(current)
}

func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func stateOf(state OrderState) stateMachineUtils.State[OrderState] {
	return stateMachineUtils.NewState(state)
}
