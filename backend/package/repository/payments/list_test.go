package paymentsRepository

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestListPaymentsAndCounts(t *testing.T) {
	r, _ := newRepo(t)
	var ids []string
	for i := range 5 {
		input := createInput(fmt.Sprintf("order-%d", i))
		if i%2 == 1 {
			input.Provider = ProviderSelfUPI
		}
		ids = append(ids, mustCreate(t, r, input).ID)
	}
	if _, _, err := r.Transition(context.Background(), ids[4], StateFailed, Patch{}, EventInput{Source: EventSourceSystem, Kind: "x"}); err != nil {
		t.Fatalf("fail: %v", err)
	}
	first, err := r.ListPayments(context.Background(), ListQuery{Limit: 2})
	if err != nil || len(first.Payments) != 2 || first.Payments[0].ID != ids[4] || first.NextCursor == "" {
		t.Fatalf("first page: %v %+v", err, first)
	}
	second, err := r.ListPayments(context.Background(), ListQuery{Limit: 2, Cursor: first.NextCursor})
	if err != nil || len(second.Payments) != 2 || second.Payments[0].ID != ids[2] {
		t.Fatalf("second page: %v", err)
	}
	third, err := r.ListPayments(context.Background(), ListQuery{Limit: 2, Cursor: second.NextCursor})
	if err != nil || len(third.Payments) != 1 || third.NextCursor != "" {
		t.Fatalf("third page: %v", err)
	}
	failed, err := r.ListPayments(context.Background(), ListQuery{States: []State{StateFailed}})
	if err != nil || len(failed.Payments) != 1 || failed.Payments[0].ID != ids[4] {
		t.Fatalf("state filter: %v", err)
	}
	upi, err := r.ListPayments(context.Background(), ListQuery{Provider: ProviderSelfUPI})
	if err != nil || len(upi.Payments) != 2 {
		t.Fatalf("provider filter: %v %d", err, len(upi.Payments))
	}
	counts, err := r.CountPaymentsByState(context.Background())
	if err != nil || counts[StateCreated] != 4 || counts[StateFailed] != 1 || counts[StateSucceeded] != 0 || len(counts) != 6 {
		t.Fatalf("counts: %v %v", err, counts)
	}
	for _, q := range []ListQuery{{Limit: 101}, {States: []State{"X"}}, {Provider: "X"}, {Cursor: "!!"}} {
		if _, err := r.ListPayments(context.Background(), q); err == nil {
			t.Fatalf("query %+v must fail", q)
		}
	}
	if _, err := r.ListPayments(context.Background(), ListQuery{Cursor: "e30"}); !errors.Is(err, ErrCursorInvalid) {
		t.Fatalf("cursor: %v", err)
	}
}

func TestListPaymentsForOrders(t *testing.T) {
	r, _ := newRepo(t)
	a := mustCreate(t, r, createInput("order-a"))
	if _, _, err := r.Transition(context.Background(), a.ID, StateFailed, Patch{}, EventInput{Source: EventSourceSystem, Kind: "x"}); err != nil {
		t.Fatal(err)
	}
	a2 := mustCreate(t, r, createInput("order-a"))
	b := mustCreate(t, r, createInput("order-b"))
	got, err := r.ListPaymentsForOrders(context.Background(), []string{"order-a", "order-b", "order-a", "order-none"}, true)
	if err != nil || len(got["order-a"]) != 2 || got["order-a"][0].ID != a2.ID || len(got["order-b"]) != 1 || got["order-b"][0].ID != b.ID || len(got["order-none"]) != 0 {
		t.Fatalf("batch: %v %+v", err, got)
	}
	if len(got["order-a"][1].Events) != 2 || len(got["order-b"][0].Events) != 1 {
		t.Fatalf("events not loaded")
	}
	lean, err := r.ListPaymentsForOrders(context.Background(), []string{"order-a"}, false)
	if err != nil || len(lean["order-a"][0].Events) != 0 {
		t.Fatalf("events must be skipped: %v", err)
	}
	empty, err := r.ListPaymentsForOrders(context.Background(), nil, false)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty: %v", err)
	}
	if _, err := r.ListPaymentsForOrders(context.Background(), []string{""}, false); !errors.Is(err, ErrOrderIDInvalid) {
		t.Fatalf("bad id: %v", err)
	}
	big := make([]string, 201)
	for i := range big {
		big[i] = fmt.Sprintf("o-%d", i)
	}
	if _, err := r.ListPaymentsForOrders(context.Background(), big, false); !errors.Is(err, ErrBatchTooLarge) {
		t.Fatalf("too many: %v", err)
	}
}
