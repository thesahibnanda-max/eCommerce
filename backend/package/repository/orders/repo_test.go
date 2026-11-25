package ordersRepository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

var bg = context.Background()

func openRepo(t *testing.T, dir string) *repository {
	t.Helper()
	r, err := NewRepository([]string{dir, "orders.sqlite"}, 0o600)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r.(*repository)
}

func newRepo(t *testing.T) *repository {
	t.Helper()
	return openRepo(t, t.TempDir())
}

func address() ShippingAddress {
	return ShippingAddress{
		AddressID:    utils.NewULID(),
		FullName:     "  Riya   Sharma ",
		PhoneNumber:  "+919876543210",
		AddressLine1: "12, MG Road",
		City:         "New Delhi",
		State:        "Delhi",
		StateCode:    "IN-DL",
		Pincode:      "110 001",
		Country:      "India",
	}
}

func item(price, quantity int64) Item {
	return Item{
		ProductID:        utils.NewULID(),
		ProductVariantID: utils.NewULID(),
		SKU:              "SKU-" + utils.NewULID()[20:],
		ProductName:      "Pastel Tee",
		Attributes:       map[string]string{"size": "M"},
		UnitPrice:        price,
		Quantity:         quantity,
	}
}

func input(userID string) CreateOrderInput {
	return CreateOrderInput{
		UserID:          userID,
		IdempotencyKey:  utils.NewULID(),
		ShippingFee:     79,
		Discount:        50,
		ShippingAddress: address(),
		Items:           []Item{item(499, 2), item(299, 1)},
		Metadata:        map[string]any{"source": "web"},
	}
}

func mustCreate(t *testing.T, r *repository, in CreateOrderInput) Model {
	t.Helper()
	order, err := r.CreateOrder(bg, in)
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	return order
}

func mustTransition(t *testing.T, r *repository, id string, to OrderState) Model {
	t.Helper()
	order, err := r.TransitionOrder(bg, id, to, ActorSystem, "test")
	if err != nil {
		t.Fatalf("TransitionOrder(%s) error = %v", to, err)
	}
	return order
}

func TestCreateOrder(t *testing.T) {
	r := newRepo(t)
	stamp := time.Date(2026, 11, 14, 10, 30, 0, 123456000, time.UTC)
	r.now = func() (time.Time, error) { return stamp, nil }
	user := utils.NewULID()
	order := mustCreate(t, r, input(user))
	if err := validateOrderID(order.ID); err != nil {
		t.Fatalf("order id %q invalid", order.ID)
	}
	if order.UserID != user || order.Currency != IndianNationalRupeeCurrency {
		t.Fatalf("order = %+v", order)
	}
	if order.Subtotal != 1297 || order.ShippingFee != 79 || order.Discount != 50 || order.TotalPayable != 1326 {
		t.Fatalf("money = %d %d %d %d", order.Subtotal, order.ShippingFee, order.Discount, order.TotalPayable)
	}
	if len(order.Items) != 2 || order.Items[0].LineTotal != 998 || order.Items[1].LineTotal != 299 || order.Items[0].Attributes["size"] != "M" {
		t.Fatalf("items = %+v", order.Items)
	}
	if order.ShippingAddress.FullName != "Riya Sharma" || order.ShippingAddress.Pincode != "110001" {
		t.Fatalf("address = %+v", order.ShippingAddress)
	}
	if order.OrderState.CurrentState().Value() != OrderStateCreated {
		t.Fatalf("state = %v", order.OrderState.CurrentState().Value())
	}
	if len(order.StateHistory) != 1 || order.StateHistory[0].ToState != OrderStateCreated || order.StateHistory[0].FromState != "" || order.StateHistory[0].Actor != ActorUser {
		t.Fatalf("history = %+v", order.StateHistory)
	}
	if !order.CreatedAt.Equal(stamp) || order.CreatedAt.Location().String() != "Asia/Kolkata" {
		t.Fatalf("created at = %v", order.CreatedAt)
	}
	if order.Metadata["source"] != "web" || len(order.PaymentEvents) != 0 {
		t.Fatalf("metadata/events = %+v %+v", order.Metadata, order.PaymentEvents)
	}
}

func TestCreateOrder_Idempotent(t *testing.T) {
	r := newRepo(t)
	in := input(utils.NewULID())
	first := mustCreate(t, r, in)
	again := mustCreate(t, r, in)
	if again.ID != first.ID {
		t.Fatalf("retry created a new order: %s vs %s", again.ID, first.ID)
	}
	changed := in
	changed.ShippingFee = 0
	if _, err := r.CreateOrder(bg, changed); !errors.Is(err, ErrIdempotencyKeyConflict) {
		t.Fatalf("changed retry error = %v", err)
	}
	other := in
	other.UserID = utils.NewULID()
	if o := mustCreate(t, r, other); o.ID == first.ID {
		t.Fatal("idempotency key leaked across users")
	}
	page, _ := r.ListUserOrders(bg, in.UserID, ListQuery{})
	if len(page.Orders) != 1 {
		t.Fatalf("orders stored = %d", len(page.Orders))
	}
}

func TestCreateOrder_IdempotentConcurrent(t *testing.T) {
	r := newRepo(t)
	in := input(utils.NewULID())
	var wg sync.WaitGroup
	ids := make(chan string, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			order, err := r.CreateOrder(bg, in)
			if err != nil {
				t.Error(err)
				return
			}
			ids <- order.ID
		}()
	}
	wg.Wait()
	close(ids)
	seen := map[string]struct{}{}
	for id := range ids {
		seen[id] = struct{}{}
	}
	if len(seen) != 1 {
		t.Fatalf("concurrent retries created %d orders", len(seen))
	}
}

func TestCreateOrder_Validation(t *testing.T) {
	r := newRepo(t)
	user := utils.NewULID()
	tests := []struct {
		name   string
		mutate func(*CreateOrderInput)
		want   error
	}{
		{"user", func(in *CreateOrderInput) { in.UserID = "x" }, ErrUserIDInvalid},
		{"key empty", func(in *CreateOrderInput) { in.IdempotencyKey = " " }, ErrIdempotencyKeyInvalid},
		{"key long", func(in *CreateOrderInput) { in.IdempotencyKey = strings.Repeat("k", 129) }, ErrIdempotencyKeyInvalid},
		{"currency", func(in *CreateOrderInput) { in.Currency = "USD" }, ErrCurrencyInvalid},
		{"no items", func(in *CreateOrderInput) { in.Items = nil }, ErrItemsRequired},
		{"too many", func(in *CreateOrderInput) {
			in.Items = nil
			for i := 0; i <= maxItems; i++ {
				in.Items = append(in.Items, item(10, 1))
			}
		}, ErrTooManyItems},
		{"bad product", func(in *CreateOrderInput) { in.Items[0].ProductID = "x" }, ErrItemInvalid},
		{"no sku", func(in *CreateOrderInput) { in.Items[0].SKU = " " }, ErrItemInvalid},
		{"no name", func(in *CreateOrderInput) { in.Items[0].ProductName = "" }, ErrItemInvalid},
		{"dup variant", func(in *CreateOrderInput) { in.Items[1].ProductVariantID = in.Items[0].ProductVariantID }, ErrDuplicateVariant},
		{"quantity", func(in *CreateOrderInput) { in.Items[0].Quantity = 0 }, ErrQuantityInvalid},
		{"price", func(in *CreateOrderInput) { in.Items[0].UnitPrice = 0 }, ErrPriceInvalid},
		{"overflow", func(in *CreateOrderInput) { in.Items[0].UnitPrice = math.MaxInt64 / 2 }, ErrAmountOverflow},
		{"negative fee", func(in *CreateOrderInput) { in.ShippingFee = -1 }, ErrAmountInvalid},
		{"negative discount", func(in *CreateOrderInput) { in.Discount = -1 }, ErrAmountInvalid},
		{"discount too big", func(in *CreateOrderInput) { in.Discount = 1297 + 79 + 1 }, ErrAmountInvalid},
		{"address name", func(in *CreateOrderInput) { in.ShippingAddress.FullName = "" }, ErrAddressIncomplete},
		{"address pincode", func(in *CreateOrderInput) { in.ShippingAddress.Pincode = "12345" }, ErrAddressIncomplete},
		{"address id", func(in *CreateOrderInput) { in.ShippingAddress.AddressID = "x" }, ErrAddressIncomplete},
		{"metadata", func(in *CreateOrderInput) { in.Metadata = map[string]any{"x": strings.Repeat("y", maxMetadataBytes)} }, ErrMetadataTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := input(user)
			tt.mutate(&in)
			if _, err := r.CreateOrder(bg, in); !errors.Is(err, tt.want) {
				t.Fatalf("CreateOrder() error = %v, want %v", err, tt.want)
			}
		})
	}
	counts, err := r.CountOrdersByState(bg)
	if err != nil || counts[OrderStateCreated] != 0 {
		t.Fatalf("invalid creates stored rows: %v %v", counts, err)
	}
}

func TestCreateOrder_FreeAfterDiscount(t *testing.T) {
	r := newRepo(t)
	in := input(utils.NewULID())
	in.Discount = 1297 + 79
	if order := mustCreate(t, r, in); order.TotalPayable != 0 {
		t.Fatalf("total = %d", order.TotalPayable)
	}
}

func TestGetOrder(t *testing.T) {
	r := newRepo(t)
	user := utils.NewULID()
	created := mustCreate(t, r, input(user))
	got, err := r.GetOrder(bg, created.ID)
	if err != nil || got.ID != created.ID || len(got.Items) != 2 || len(got.StateHistory) != 1 {
		t.Fatalf("GetOrder() = %+v, %v", got, err)
	}
	if _, err := r.GetUserOrder(bg, user, created.ID); err != nil {
		t.Fatalf("GetUserOrder() owner error = %v", err)
	}
	if _, err := r.GetUserOrder(bg, utils.NewULID(), created.ID); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("other user error = %v", err)
	}
	missing, _ := newOrderID()
	if _, err := r.GetOrder(bg, missing); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("missing error = %v", err)
	}
	for _, bad := range []string{"", "nope", strings.Replace(created.ID, "-", "_", 1), created.ID[:62]} {
		if _, err := r.GetOrder(bg, bad); !errors.Is(err, ErrOrderIDInvalid) {
			t.Fatalf("GetOrder(%q) error = %v", bad, err)
		}
	}
	if _, err := r.GetUserOrder(bg, "x", created.ID); !errors.Is(err, ErrUserIDInvalid) {
		t.Fatalf("bad user error = %v", err)
	}
}

func TestOrderJSONRoundTrip(t *testing.T) {
	r := newRepo(t)
	order := mustCreate(t, r, input(utils.NewULID()))
	raw, err := utils.FromObjectToStringJSON(order)
	if err != nil {
		t.Fatalf("marshal error = %v", err)
	}
	back, err := utils.FromStringToObjectJSON[Model](raw)
	if err != nil || back.ID != order.ID || back.OrderState.CurrentState().Value() != OrderStateCreated || len(back.Items) != 2 {
		t.Fatalf("round trip = %+v, %v", back, err)
	}
	if !back.OrderState.CanTransitionTo(stateOf(OrderStateProcessing)) {
		t.Fatal("decoded machine lost transitions")
	}
}

func TestListOrders_Pagination(t *testing.T) {
	r := newRepo(t)
	user := utils.NewULID()
	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	tick := 0
	r.now = func() (time.Time, error) {
		tick++
		return base.Add(time.Duration(tick/3) * time.Minute), nil
	}
	var ids []string
	for i := 0; i < 7; i++ {
		ids = append(ids, mustCreate(t, r, input(user)).ID)
	}
	mustCreate(t, r, input(utils.NewULID()))
	var got []string
	cursor := ""
	pages := 0
	for {
		page, err := r.ListUserOrders(bg, user, ListQuery{Limit: 3, Cursor: cursor})
		if err != nil {
			t.Fatalf("ListUserOrders() error = %v", err)
		}
		pages++
		for _, o := range page.Orders {
			if len(o.Items) != 2 || o.UserID != user {
				t.Fatalf("listed order = %+v", o)
			}
			got = append(got, o.ID)
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if pages != 3 || len(got) != 7 {
		t.Fatalf("pages = %d, got %d orders", pages, len(got))
	}
	seen := map[string]bool{}
	for i, id := range got {
		if seen[id] {
			t.Fatalf("duplicate %s", id)
		}
		seen[id] = true
		if i > 0 && strings.Compare(got[i-1], id) < 0 {
			created := func(x string) time.Time { o, _ := r.GetOrder(bg, x); return o.CreatedAt }
			if !created(got[i-1]).After(created(id)) {
				t.Fatalf("not newest first at %d", i)
			}
		}
	}
	all, err := r.ListOrders(bg, ListQuery{Limit: 100})
	if err != nil || len(all.Orders) != 8 || all.NextCursor != "" {
		t.Fatalf("ListOrders() = %d %q %v", len(all.Orders), all.NextCursor, err)
	}
}

func TestListOrders_StateFilter(t *testing.T) {
	r := newRepo(t)
	user := utils.NewULID()
	a := mustCreate(t, r, input(user))
	b := mustCreate(t, r, input(user))
	mustCreate(t, r, input(user))
	mustTransition(t, r, a.ID, OrderStateProcessing)
	mustTransition(t, r, b.ID, OrderStateCancelled)
	page, err := r.ListUserOrders(bg, user, ListQuery{States: []OrderState{OrderStateProcessing, OrderStateCancelled}})
	if err != nil || len(page.Orders) != 2 {
		t.Fatalf("filtered = %d, %v", len(page.Orders), err)
	}
	admin, err := r.ListOrders(bg, ListQuery{States: []OrderState{OrderStateCreated}})
	if err != nil || len(admin.Orders) != 1 {
		t.Fatalf("admin filtered = %d, %v", len(admin.Orders), err)
	}
	counts, err := r.CountOrdersByState(bg)
	if err != nil || counts[OrderStateCreated] != 1 || counts[OrderStateProcessing] != 1 || counts[OrderStateCancelled] != 1 || counts[OrderStateRefunded] != 0 || len(counts) != len(orderStates) {
		t.Fatalf("counts = %v, %v", counts, err)
	}
}

func TestListOrders_Rejects(t *testing.T) {
	r := newRepo(t)
	user := utils.NewULID()
	tests := []struct {
		query ListQuery
		want  error
	}{
		{ListQuery{Limit: -1}, ErrLimitInvalid},
		{ListQuery{Limit: maxLimit + 1}, ErrLimitInvalid},
		{ListQuery{States: []OrderState{"NOPE"}}, ErrStateInvalid},
		{ListQuery{Cursor: "!!"}, ErrCursorInvalid},
		{ListQuery{Cursor: "e30"}, ErrCursorInvalid},
	}
	for _, tt := range tests {
		if _, err := r.ListUserOrders(bg, user, tt.query); !errors.Is(err, tt.want) {
			t.Fatalf("ListUserOrders(%+v) error = %v, want %v", tt.query, err, tt.want)
		}
		if _, err := r.ListOrders(bg, tt.query); !errors.Is(err, tt.want) {
			t.Fatalf("ListOrders(%+v) error = %v, want %v", tt.query, err, tt.want)
		}
	}
	if _, err := r.ListUserOrders(bg, "x", ListQuery{}); !errors.Is(err, ErrUserIDInvalid) {
		t.Fatalf("bad user error = %v", err)
	}
	empty, err := r.ListUserOrders(bg, user, ListQuery{})
	if err != nil || empty.Orders == nil || len(empty.Orders) != 0 || empty.NextCursor != "" {
		t.Fatalf("empty page = %+v, %v", empty, err)
	}
}

func TestTransitionOrder_Graph(t *testing.T) {
	r := newRepo(t)
	user := utils.NewULID()
	for _, from := range orderStates {
		for _, to := range orderStates {
			allowed := false
			for _, next := range orderTransitions[from] {
				allowed = allowed || next == to
			}
			order := mustCreate(t, r, input(user))
			if err := driveTo(t, r, order.ID, from); err != nil {
				t.Fatalf("cannot reach %s: %v", from, err)
			}
			_, err := r.TransitionOrder(bg, order.ID, to, ActorAdmin, "graph")
			if allowed && err != nil {
				t.Fatalf("%s -> %s rejected: %v", from, to, err)
			}
			if !allowed && !errors.Is(err, ErrTransitionNotAllowed) {
				t.Fatalf("%s -> %s error = %v", from, to, err)
			}
		}
	}
}

func driveTo(t *testing.T, r *repository, id string, target OrderState) error {
	t.Helper()
	paths := map[OrderState][]OrderState{
		OrderStateCreated:         {},
		OrderStateDitched:         {OrderStateDitched},
		OrderStateProcessing:      {OrderStateProcessing},
		OrderStateApproved:        {OrderStateProcessing, OrderStateApproved},
		OrderStatePacked:          {OrderStateProcessing, OrderStateApproved, OrderStatePacked},
		OrderStateStartedShipping: {OrderStateProcessing, OrderStateApproved, OrderStateStartedShipping},
		OrderStateOutForDelivery:  {OrderStateProcessing, OrderStateApproved, OrderStateStartedShipping, OrderStateOutForDelivery},
		OrderStateDelivered:       {OrderStateProcessing, OrderStateApproved, OrderStateStartedShipping, OrderStateDelivered},
		OrderStateCancelled:       {OrderStateCancelled},
		OrderStateReturned:        {OrderStateProcessing, OrderStateApproved, OrderStateStartedShipping, OrderStateReturned},
		OrderStateRefunded:        {OrderStateCancelled, OrderStateRefunded},
		OrderStateFailed:          {OrderStateFailed},
		OrderStateOnHold:          {OrderStateProcessing, OrderStateOnHold},
	}
	for _, step := range paths[target] {
		if _, err := r.TransitionOrder(bg, id, step, ActorSystem, ""); err != nil {
			return err
		}
	}
	return nil
}

func TestTransitionOrder_History(t *testing.T) {
	r := newRepo(t)
	order := mustCreate(t, r, input(utils.NewULID()))
	mustTransition(t, r, order.ID, OrderStateProcessing)
	got, err := r.TransitionOrder(bg, order.ID, OrderStateApproved, ActorGateway, "  payment   captured ")
	if err != nil {
		t.Fatal(err)
	}
	if got.OrderState.CurrentState().Value() != OrderStateApproved || len(got.StateHistory) != 3 {
		t.Fatalf("got = %v %d", got.OrderState.CurrentState().Value(), len(got.StateHistory))
	}
	last := got.StateHistory[2]
	if last.FromState != OrderStateProcessing || last.ToState != OrderStateApproved || last.Actor != ActorGateway || last.Reason != "payment captured" {
		t.Fatalf("last change = %+v", last)
	}
	if !got.UpdatedAt.After(got.CreatedAt) && !got.UpdatedAt.Equal(got.CreatedAt) {
		t.Fatal("updated_at went backwards")
	}
}

func TestTransitionOrder_Rejects(t *testing.T) {
	r := newRepo(t)
	order := mustCreate(t, r, input(utils.NewULID()))
	missing, _ := newOrderID()
	tests := []struct {
		id     string
		to     OrderState
		actor  Actor
		reason string
		want   error
	}{
		{"bad", OrderStateProcessing, ActorSystem, "", ErrOrderIDInvalid},
		{missing, OrderStateProcessing, ActorSystem, "", ErrOrderNotFound},
		{order.ID, "NOPE", ActorSystem, "", ErrStateInvalid},
		{order.ID, OrderStateProcessing, "ROBOT", "", ErrActorInvalid},
		{order.ID, OrderStateProcessing, ActorSystem, strings.Repeat("r", maxReasonRunes+1), ErrReasonTooLong},
		{order.ID, OrderStateCreated, ActorSystem, "", ErrTransitionNotAllowed},
	}
	for _, tt := range tests {
		if _, err := r.TransitionOrder(bg, tt.id, tt.to, tt.actor, tt.reason); !errors.Is(err, tt.want) {
			t.Fatalf("TransitionOrder(%s,%s,%s) error = %v, want %v", tt.id, tt.to, tt.actor, err, tt.want)
		}
	}
	got, _ := r.GetOrder(bg, order.ID)
	if got.OrderState.CurrentState().Value() != OrderStateCreated || len(got.StateHistory) != 1 {
		t.Fatal("rejected transitions changed the order")
	}
}

func TestTransitionOrder_ConcurrentOneWinner(t *testing.T) {
	r := newRepo(t)
	for round := 0; round < 10; round++ {
		order := mustCreate(t, r, input(utils.NewULID()))
		var wg sync.WaitGroup
		results := make(chan error, 2)
		for _, to := range []OrderState{OrderStateCancelled, OrderStateFailed} {
			wg.Add(1)
			go func(to OrderState) {
				defer wg.Done()
				_, err := r.TransitionOrder(bg, order.ID, to, ActorSystem, "race")
				results <- err
			}(to)
		}
		wg.Wait()
		close(results)
		wins := 0
		for err := range results {
			switch {
			case err == nil:
				wins++
			case !errors.Is(err, ErrTransitionNotAllowed):
				t.Fatalf("unexpected error = %v", err)
			}
		}
		got, _ := r.GetOrder(bg, order.ID)
		if wins != 1 || len(got.StateHistory) != 2 {
			t.Fatalf("round %d: wins = %d, history = %d", round, wins, len(got.StateHistory))
		}
	}
}

func TestSetPaymentGateway(t *testing.T) {
	r := newRepo(t)
	order := mustCreate(t, r, input(utils.NewULID()))
	got, err := r.SetPaymentGateway(bg, order.ID, " cashfree ")
	if err != nil || got.PaymentGateway != "cashfree" {
		t.Fatalf("SetPaymentGateway() = %q, %v", got.PaymentGateway, err)
	}
	if _, err := r.SetPaymentGateway(bg, order.ID, ""); !errors.Is(err, ErrPaymentGatewayInvalid) {
		t.Fatalf("empty gateway error = %v", err)
	}
	mustTransition(t, r, order.ID, OrderStateProcessing)
	if got, err := r.SetPaymentGateway(bg, order.ID, "phonepe"); err != nil || got.PaymentGateway != "phonepe" {
		t.Fatalf("processing switch = %v", err)
	}
	mustTransition(t, r, order.ID, OrderStateApproved)
	if _, err := r.SetPaymentGateway(bg, order.ID, "cashfree"); !errors.Is(err, ErrPaymentGatewayLocked) {
		t.Fatalf("locked error = %v", err)
	}
}

func TestRecordPaymentEvent(t *testing.T) {
	r := newRepo(t)
	order := mustCreate(t, r, input(utils.NewULID()))
	event := PaymentEvent{
		Gateway:          "cashfree",
		GatewayEventID:   "evt_1",
		EventType:        "PAYMENT_SUCCESS",
		GatewayOrderID:   "cf_order_1",
		GatewayPaymentID: "cf_pay_1",
		Status:           "SUCCESS",
		Amount:           132600,
		Raw:              map[string]any{"cf_payment_id": "cf_pay_1", "nested": map[string]any{"ok": true}},
	}
	got, err := r.RecordPaymentEvent(bg, order.ID, event)
	if err != nil || len(got.PaymentEvents) != 1 {
		t.Fatalf("RecordPaymentEvent() = %v", err)
	}
	stored := got.PaymentEvents[0]
	if stored.GatewayPaymentID != "cf_pay_1" || stored.Amount != 132600 || stored.Raw["nested"].(map[string]any)["ok"] != true {
		t.Fatalf("stored = %+v", stored)
	}
	if _, err := r.RecordPaymentEvent(bg, order.ID, event); !errors.Is(err, ErrPaymentEventDuplicate) {
		t.Fatalf("duplicate error = %v", err)
	}
	other := mustCreate(t, r, input(utils.NewULID()))
	if _, err := r.RecordPaymentEvent(bg, other.ID, event); !errors.Is(err, ErrPaymentEventDuplicate) {
		t.Fatalf("cross-order duplicate error = %v", err)
	}
	event.Gateway = "phonepe"
	if _, err := r.RecordPaymentEvent(bg, order.ID, event); err != nil {
		t.Fatalf("same id other gateway = %v", err)
	}
	missing, _ := newOrderID()
	if _, err := r.RecordPaymentEvent(bg, missing, PaymentEvent{Gateway: "x", GatewayEventID: "y", EventType: "z"}); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("missing order error = %v", err)
	}
	for _, bad := range []PaymentEvent{{GatewayEventID: "a", EventType: "b"}, {Gateway: "a", EventType: "b"}, {Gateway: "a", GatewayEventID: "b"}, {Gateway: "a", GatewayEventID: "b", EventType: "c", Amount: -1}} {
		if _, err := r.RecordPaymentEvent(bg, order.ID, bad); !errors.Is(err, ErrPaymentEventInvalid) {
			t.Fatalf("invalid event %+v error = %v", bad, err)
		}
	}
}

func TestUpdateMetadata(t *testing.T) {
	r := newRepo(t)
	order := mustCreate(t, r, input(utils.NewULID()))
	got, err := r.UpdateMetadata(bg, order.ID, func(m map[string]any) (map[string]any, error) {
		m["tracking"] = "AWB123"
		return m, nil
	})
	if err != nil || got.Metadata["tracking"] != "AWB123" || got.Metadata["source"] != "web" {
		t.Fatalf("UpdateMetadata() = %v, %v", got.Metadata, err)
	}
	if _, err := r.UpdateMetadata(bg, order.ID, nil); !errors.Is(err, ErrMutateRequired) {
		t.Fatalf("nil mutate = %v", err)
	}
	if _, err := r.UpdateMetadata(bg, order.ID, func(map[string]any) (map[string]any, error) { panic("boom") }); !errors.Is(err, ErrMutatePanicked) {
		t.Fatalf("panic = %v", err)
	}
	boom := errors.New("boom")
	if _, err := r.UpdateMetadata(bg, order.ID, func(map[string]any) (map[string]any, error) { return nil, boom }); !errors.Is(err, boom) {
		t.Fatalf("mutate error = %v", err)
	}
	if _, err := r.UpdateMetadata(bg, order.ID, func(map[string]any) (map[string]any, error) {
		return map[string]any{"x": strings.Repeat("y", maxMetadataBytes)}, nil
	}); !errors.Is(err, ErrMetadataTooLarge) {
		t.Fatalf("too large = %v", err)
	}
	cleared, err := r.UpdateMetadata(bg, order.ID, func(map[string]any) (map[string]any, error) { return nil, nil })
	if err != nil || cleared.Metadata == nil || len(cleared.Metadata) != 0 {
		t.Fatalf("cleared = %v, %v", cleared.Metadata, err)
	}
}

func TestCascadeDelete(t *testing.T) {
	r := newRepo(t)
	order := mustCreate(t, r, input(utils.NewULID()))
	if _, err := r.RecordPaymentEvent(bg, order.ID, PaymentEvent{Gateway: "g", GatewayEventID: "e", EventType: "t"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.writer.Exec("DELETE FROM orders WHERE id = ?", order.ID); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"order_items", "order_state_history", "order_payment_events"} {
		var n int
		if err := r.reader.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE order_id = ?", table), order.ID).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s left %d rows, %v", table, n, err)
		}
	}
}

func TestSchemaChecksBackstop(t *testing.T) {
	r := newRepo(t)
	order := mustCreate(t, r, input(utils.NewULID()))
	bad := []string{
		"UPDATE orders SET state = 'LOST' WHERE id = ?",
		"UPDATE orders SET total_payable = total_payable + 1 WHERE id = ?",
		"UPDATE order_items SET line_total = line_total + 1 WHERE order_id = ?",
		"UPDATE orders SET currency = 'USD' WHERE id = ?",
	}
	for _, stmt := range bad {
		if _, err := r.writer.Exec(stmt, order.ID); err == nil {
			t.Fatalf("schema accepted %q", stmt)
		}
	}
	if _, err := r.reader.Exec("UPDATE orders SET metadata = '{}' WHERE id = ?", order.ID); err == nil {
		t.Fatal("reader pool can write")
	}
}

func TestReopenKeepsData(t *testing.T) {
	dir := t.TempDir()
	first, err := NewRepository([]string{dir, "orders.sqlite"}, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	order, err := first.CreateOrder(bg, input(utils.NewULID()))
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second := openRepo(t, dir)
	got, err := second.GetOrder(bg, order.ID)
	if err != nil || got.TotalPayable != order.TotalPayable || len(got.Items) != 2 {
		t.Fatalf("reopened = %+v, %v", got, err)
	}
}

func TestContextChecked(t *testing.T) {
	r := newRepo(t)
	ctx, cancel := context.WithCancel(bg)
	cancel()
	id, _ := newOrderID()
	user := utils.NewULID()
	mutate := func(m map[string]any) (map[string]any, error) { return m, nil }
	calls := []func() error{
		func() error { _, err := r.CreateOrder(ctx, input(user)); return err },
		func() error { _, err := r.GetOrder(ctx, id); return err },
		func() error { _, err := r.GetUserOrder(ctx, user, id); return err },
		func() error { _, err := r.ListUserOrders(ctx, user, ListQuery{}); return err },
		func() error { _, err := r.ListOrders(ctx, ListQuery{}); return err },
		func() error { _, err := r.TransitionOrder(ctx, id, OrderStateProcessing, ActorSystem, ""); return err },
		func() error { _, err := r.SetPaymentGateway(ctx, id, "g"); return err },
		func() error { _, err := r.RecordPaymentEvent(ctx, id, PaymentEvent{}); return err },
		func() error { _, err := r.UpdateMetadata(ctx, id, mutate); return err },
		func() error { _, err := r.CountOrdersByState(ctx); return err },
	}
	for i, call := range calls {
		if err := call(); err == nil {
			t.Fatalf("call %d accepted a cancelled context", i)
		}
	}
}

func TestEnums(t *testing.T) {
	if _, err := ParseOrderState("CREATED"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseOrderState("created"); !errors.Is(err, ErrStateInvalid) {
		t.Fatalf("lowercase error = %v", err)
	}
	if !OrderStateRefunded.IsTerminal() || OrderStateCreated.IsTerminal() || OrderState("X").IsTerminal() {
		t.Fatal("IsTerminal wrong")
	}
	states := OrderStates()
	states[0] = "X"
	if OrderStates()[0] != OrderStateCreated {
		t.Fatal("OrderStates exposes internal slice")
	}
	if _, err := NewOrderStateMachine("X"); !errors.Is(err, ErrStateInvalid) {
		t.Fatalf("machine error = %v", err)
	}
}

func TestRequestHash_StableAcrossMapOrder(t *testing.T) {
	in := input(utils.NewULID())
	in.Items[0].Attributes = map[string]string{"size": "M", "color": "pink", "fit": "oversized", "fabric": "cotton", "sleeve": "half"}
	normalized, _, _, err := normalizeInput(in)
	if err != nil {
		t.Fatal(err)
	}
	first, err := requestHash(normalized)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		again, _, _, _ := normalizeInput(in)
		if got, _ := requestHash(again); got != first {
			t.Fatalf("hash changed between runs: %s vs %s", got, first)
		}
	}
	r := newRepo(t)
	created := mustCreate(t, r, in)
	if retried := mustCreate(t, r, in); retried.ID != created.ID {
		t.Fatal("retry with multi-attribute items was not idempotent")
	}
}
