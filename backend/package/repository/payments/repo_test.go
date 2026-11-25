package paymentsRepository

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

var (
	jpegImage = append([]byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}, bytes.Repeat([]byte{0x01}, 64)...)
	pngImage  = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0x02}, 64)...)
)

func newRepo(t *testing.T) (*repository, string) {
	t.Helper()
	dir := t.TempDir()
	repo, err := NewRepository([]string{dir, "payments.db"}, 0o600)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo.(*repository), dir
}

func createInput(orderID string) CreateInput {
	return CreateInput{OrderID: orderID, UserID: "user-1", Provider: ProviderInstamojo, Amount: 1499, Purpose: "Order " + orderID, Metadata: map[string]any{"source": "test"}}
}

func mustCreate(t *testing.T, r Repository, input CreateInput) Model {
	t.Helper()
	model, created, err := r.CreatePayment(context.Background(), input)
	if err != nil || !created {
		t.Fatalf("CreatePayment: %v created=%v", err, created)
	}
	return model
}

func strPtr(value string) *string {
	return &value
}

func TestCreatePayment(t *testing.T) {
	r, _ := newRepo(t)
	model := mustCreate(t, r, createInput("order-1"))
	if !utils.ValidateULID(model.ID) || model.State != StateCreated || model.Amount != 1499 || model.Currency != CurrencyINR || model.Metadata["source"] != "test" {
		t.Fatalf("model %+v", model)
	}
	if len(model.Events) != 1 || model.Events[0].Kind != eventKindCreated || model.Events[0].ToState != StateCreated || model.Events[0].FromState != "" {
		t.Fatalf("events %+v", model.Events)
	}
	again, created, err := r.CreatePayment(context.Background(), createInput("order-1"))
	if err != nil || created || again.ID != model.ID {
		t.Fatalf("idempotent create: %v created=%v id=%s", err, created, again.ID)
	}
	conflict := createInput("order-1")
	conflict.Amount = 1500
	if _, _, err := r.CreatePayment(context.Background(), conflict); !errors.Is(err, ErrOpenPaymentConflict) {
		t.Fatalf("amount conflict: %v", err)
	}
	conflict = createInput("order-1")
	conflict.Provider = ProviderSelfUPI
	if _, _, err := r.CreatePayment(context.Background(), conflict); !errors.Is(err, ErrOpenPaymentConflict) {
		t.Fatalf("provider conflict: %v", err)
	}
	conflict = createInput("order-1")
	conflict.UserID = "user-2"
	if _, _, err := r.CreatePayment(context.Background(), conflict); !errors.Is(err, ErrOpenPaymentConflict) {
		t.Fatalf("user conflict: %v", err)
	}
}

func TestCreatePayment_NewAttemptAfterClose(t *testing.T) {
	r, _ := newRepo(t)
	first := mustCreate(t, r, createInput("order-1"))
	if _, _, err := r.Transition(context.Background(), first.ID, StateFailed, Patch{}, EventInput{Source: EventSourceSystem, Kind: "create_failed"}); err != nil {
		t.Fatalf("fail: %v", err)
	}
	second := mustCreate(t, r, createInput("order-1"))
	if second.ID == first.ID {
		t.Fatalf("closed payment must allow a new attempt")
	}
	list, err := r.ListOrderPayments(context.Background(), "order-1")
	if err != nil || len(list) != 2 || list[0].ID != second.ID || list[1].State != StateFailed || len(list[0].Events) != 0 {
		t.Fatalf("list %v %+v", err, list)
	}
}

func TestCreatePayment_Validation(t *testing.T) {
	r, _ := newRepo(t)
	cases := []struct {
		name   string
		mutate func(*CreateInput)
		want   error
	}{
		{"order empty", func(c *CreateInput) { c.OrderID = " " }, ErrOrderIDInvalid},
		{"order long", func(c *CreateInput) { c.OrderID = strings.Repeat("a", 129) }, ErrOrderIDInvalid},
		{"user control", func(c *CreateInput) { c.UserID = "a\x00" }, ErrUserIDInvalid},
		{"provider", func(c *CreateInput) { c.Provider = "PAYPAL" }, ErrProviderInvalid},
		{"amount", func(c *CreateInput) { c.Amount = 0 }, ErrAmountInvalid},
		{"purpose", func(c *CreateInput) { c.Purpose = strings.Repeat("p", 101) }, ErrPurposeInvalid},
		{"metadata", func(c *CreateInput) { c.Metadata = map[string]any{"x": strings.Repeat("m", 9000)} }, ErrMetadataTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := createInput("order-v")
			tc.mutate(&input)
			if _, _, err := r.CreatePayment(context.Background(), input); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestTransition_Graph(t *testing.T) {
	r, _ := newRepo(t)
	allowed := map[State][]State{}
	for from, targets := range stateTransitions {
		allowed[from] = targets
	}
	states := []State{StateCreated, StateAwaitingPayment, StateVerifying, StateSucceeded, StateFailed, StateExpired}
	paths := map[State][]State{
		StateCreated:         {},
		StateAwaitingPayment: {StateAwaitingPayment},
		StateVerifying:       {StateAwaitingPayment, StateVerifying},
		StateSucceeded:       {StateAwaitingPayment, StateSucceeded},
		StateFailed:          {StateFailed},
		StateExpired:         {StateAwaitingPayment, StateExpired},
	}
	n := 0
	for _, from := range states {
		for _, to := range states {
			if from == to {
				continue
			}
			n++
			model := mustCreate(t, r, createInput("graph-"+from.String()+"-"+to.String()))
			for _, step := range paths[from] {
				if _, _, err := r.Transition(context.Background(), model.ID, step, Patch{}, EventInput{Source: EventSourceSystem, Kind: "setup"}); err != nil {
					t.Fatalf("setup %s->%s: %v", from, step, err)
				}
			}
			want := false
			for _, target := range allowed[from] {
				if target == to {
					want = true
				}
			}
			_, moved, err := r.Transition(context.Background(), model.ID, to, Patch{}, EventInput{Source: EventSourceSystem, Kind: "check"})
			if want && (err != nil || !moved) {
				t.Fatalf("%s->%s should be allowed: %v", from, to, err)
			}
			if !want && !errors.Is(err, ErrTransitionNotAllowed) {
				t.Fatalf("%s->%s should be blocked: %v", from, to, err)
			}
		}
	}
	if n != 30 {
		t.Fatalf("checked %d pairs", n)
	}
}

func TestTransition_PatchEventsAndIdempotency(t *testing.T) {
	r, _ := newRepo(t)
	model := mustCreate(t, r, createInput("order-1"))
	patch := Patch{GatewayReference: strPtr("ref-1"), CheckoutURL: strPtr("https://pay.example.com/ref-1"), Metadata: map[string]any{"deadline": "x", "source": nil}}
	updated, moved, err := r.Transition(context.Background(), model.ID, StateAwaitingPayment, patch, EventInput{Source: EventSourceSystem, Kind: "awaiting_payment", Data: map[string]any{"k": "v"}, DedupeKey: "await"})
	if err != nil || !moved {
		t.Fatalf("transition: %v", err)
	}
	if updated.GatewayReference != "ref-1" || updated.CheckoutURL != "https://pay.example.com/ref-1" || updated.Metadata["deadline"] != "x" || updated.Metadata["source"] != nil {
		t.Fatalf("patch not applied %+v", updated)
	}
	last := updated.Events[len(updated.Events)-1]
	if last.FromState != StateCreated || last.ToState != StateAwaitingPayment || last.Data["k"] != "v" || last.Source != EventSourceSystem {
		t.Fatalf("event %+v", last)
	}
	same, moved, err := r.Transition(context.Background(), model.ID, StateAwaitingPayment, Patch{}, EventInput{Source: EventSourceSystem, Kind: "awaiting_payment", DedupeKey: "await"})
	if err != nil || moved || len(same.Events) != len(updated.Events) {
		t.Fatalf("same-state transition must be a no-op: %v moved=%v", err, moved)
	}
	if _, _, err := r.Transition(context.Background(), model.ID, StateSucceeded, Patch{}, EventInput{Source: EventSourceSystem, Kind: "dup", DedupeKey: "await"}); !errors.Is(err, ErrEventDuplicate) {
		t.Fatalf("dedupe: %v", err)
	}
	got, err := r.GetPaymentByReference(context.Background(), ProviderInstamojo, " ref-1 ")
	if err != nil || got.ID != model.ID {
		t.Fatalf("by reference: %v", err)
	}
	if _, err := r.GetPaymentByReference(context.Background(), ProviderSelfUPI, "ref-1"); !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf("other provider: %v", err)
	}
}

func TestTransition_ConcurrentOneWinner(t *testing.T) {
	r, _ := newRepo(t)
	for i := range 10 {
		model := mustCreate(t, r, createInput("race-"+string(rune('a'+i))))
		if _, _, err := r.Transition(context.Background(), model.ID, StateAwaitingPayment, Patch{}, EventInput{Source: EventSourceSystem, Kind: "await"}); err != nil {
			t.Fatalf("await: %v", err)
		}
		var winners atomic.Int64
		var wg sync.WaitGroup
		for _, to := range []State{StateSucceeded, StateSucceeded, StateSucceeded, StateExpired} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, moved, err := r.Transition(context.Background(), model.ID, to, Patch{}, EventInput{Source: EventSourcePoll, Kind: "close"})
				if err != nil && !errors.Is(err, ErrTransitionNotAllowed) {
					t.Errorf("transition: %v", err)
				}
				if moved && to == StateSucceeded {
					winners.Add(1)
				}
			}()
		}
		wg.Wait()
		final, err := r.GetPayment(context.Background(), model.ID)
		if err != nil || final.State != StateSucceeded || winners.Load() != 1 {
			t.Fatalf("round %d: state %s winners %d err %v", i, final.State, winners.Load(), err)
		}
	}
}

func TestUpdate(t *testing.T) {
	r, _ := newRepo(t)
	a := mustCreate(t, r, createInput("order-a"))
	b := mustCreate(t, r, createInput("order-b"))
	updated, err := r.Update(context.Background(), a.ID, Patch{GatewayReference: strPtr("ref-a"), GatewayPaymentID: strPtr("MOJO1"), FailureReason: strPtr("declined")}, EventInput{Source: EventSourceWebhook, Kind: "webhook_received", DedupeKey: "w1"})
	if err != nil || updated.GatewayReference != "ref-a" || updated.GatewayPaymentID != "MOJO1" || updated.FailureReason != "declined" || updated.State != StateCreated {
		t.Fatalf("update: %v %+v", err, updated)
	}
	if _, err := r.Update(context.Background(), a.ID, Patch{}, EventInput{Source: EventSourceWebhook, Kind: "webhook_received", DedupeKey: "w1"}); !errors.Is(err, ErrEventDuplicate) {
		t.Fatalf("dup: %v", err)
	}
	if _, err := r.Update(context.Background(), b.ID, Patch{GatewayReference: strPtr("ref-a")}, EventInput{Source: EventSourceSystem, Kind: "steal"}); !errors.Is(err, ErrReferenceTaken) {
		t.Fatalf("reference taken: %v", err)
	}
	if _, err := r.Update(context.Background(), a.ID, Patch{GatewayReference: strPtr("ref-a")}, EventInput{Source: EventSourceSystem, Kind: "same_ref"}); err != nil {
		t.Fatalf("same reference again: %v", err)
	}
	bad := []struct {
		patch Patch
		event EventInput
		want  error
	}{
		{Patch{CheckoutURL: strPtr("javascript:alert(1)")}, EventInput{Source: EventSourceSystem, Kind: "x"}, ErrReferenceInvalid},
		{Patch{GatewayReference: strPtr(" ")}, EventInput{Source: EventSourceSystem, Kind: "x"}, ErrReferenceInvalid},
		{Patch{GatewayPaymentID: strPtr(strings.Repeat("m", 129))}, EventInput{Source: EventSourceSystem, Kind: "x"}, ErrReferenceInvalid},
		{Patch{FailureReason: strPtr(strings.Repeat("f", 501))}, EventInput{Source: EventSourceSystem, Kind: "x"}, ErrFailureReasonTooLong},
		{Patch{Metadata: map[string]any{"big": strings.Repeat("m", 9000)}}, EventInput{Source: EventSourceSystem, Kind: "x"}, ErrMetadataTooLarge},
		{Patch{}, EventInput{Source: "ROBOT", Kind: "x"}, ErrEventSourceInvalid},
		{Patch{}, EventInput{Source: EventSourceSystem, Kind: "Bad Kind"}, ErrEventKindInvalid},
		{Patch{}, EventInput{Source: EventSourceSystem, Kind: "x", DedupeKey: strings.Repeat("d", 257)}, ErrEventDedupeKeyInvalid},
		{Patch{}, EventInput{Source: EventSourceSystem, Kind: "x", Data: map[string]any{"d": strings.Repeat("d", 17000)}}, ErrEventDataTooLarge},
	}
	for i, tc := range bad {
		if _, err := r.Update(context.Background(), a.ID, tc.patch, tc.event); !errors.Is(err, tc.want) {
			t.Fatalf("case %d: got %v, want %v", i, err, tc.want)
		}
	}
}

func TestSubmitProof(t *testing.T) {
	r, _ := newRepo(t)
	model := mustCreate(t, r, createInput("order-1"))
	proof := ProofInput{TransactionID: " 412345678901 ", Image: jpegImage}
	if _, err := r.SubmitProof(context.Background(), model.ID, proof); !errors.Is(err, ErrProofNotAllowed) {
		t.Fatalf("proof in CREATED: %v", err)
	}
	if _, _, err := r.Transition(context.Background(), model.ID, StateAwaitingPayment, Patch{}, EventInput{Source: EventSourceSystem, Kind: "await"}); err != nil {
		t.Fatalf("await: %v", err)
	}
	updated, err := r.SubmitProof(context.Background(), model.ID, proof)
	if err != nil || updated.State != StateVerifying || updated.Proof == nil || updated.Proof.TransactionID != "412345678901" || updated.Proof.ImageMIMEType != "image/jpeg" || updated.Proof.SubmittedAt.IsZero() {
		t.Fatalf("proof: %v %+v", err, updated)
	}
	updated, err = r.SubmitProof(context.Background(), model.ID, ProofInput{TransactionID: "412345678902", Image: pngImage})
	if err != nil || updated.State != StateVerifying || updated.Proof.ImageMIMEType != "image/png" {
		t.Fatalf("resubmit: %v", err)
	}
	image, mime, err := r.GetProofImage(context.Background(), model.ID)
	if err != nil || mime != "image/png" || !bytes.Equal(image, pngImage) {
		t.Fatalf("image: %v %s", err, mime)
	}
	other := mustCreate(t, r, createInput("order-2"))
	if _, _, err := r.Transition(context.Background(), other.ID, StateAwaitingPayment, Patch{}, EventInput{Source: EventSourceSystem, Kind: "await"}); err != nil {
		t.Fatalf("await: %v", err)
	}
	if _, err := r.SubmitProof(context.Background(), other.ID, ProofInput{TransactionID: "412345678902", Image: jpegImage}); !errors.Is(err, ErrProofTransactionReused) {
		t.Fatalf("reused: %v", err)
	}
	if _, _, err := r.GetProofImage(context.Background(), other.ID); !errors.Is(err, ErrProofNotFound) {
		t.Fatalf("no proof: %v", err)
	}
	bad := []struct {
		proof ProofInput
		want  error
	}{
		{ProofInput{TransactionID: "12345", Image: jpegImage}, ErrProofTransactionID},
		{ProofInput{TransactionID: "1234-5678", Image: jpegImage}, ErrProofTransactionID},
		{ProofInput{TransactionID: "123456", Image: nil}, ErrProofImageInvalid},
		{ProofInput{TransactionID: "123456", Image: []byte("<html>not an image</html>")}, ErrProofImageInvalid},
		{ProofInput{TransactionID: "123456", Image: append(append([]byte{}, jpegImage...), make([]byte, maxProofImageBytes)...)}, ErrProofImageTooLarge},
	}
	for i, tc := range bad {
		if _, err := r.SubmitProof(context.Background(), other.ID, tc.proof); !errors.Is(err, tc.want) {
			t.Fatalf("case %d: got %v, want %v", i, err, tc.want)
		}
	}
	if _, _, err := r.Transition(context.Background(), model.ID, StateSucceeded, Patch{}, EventInput{Source: EventSourceAdmin, Kind: "approved"}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, err := r.SubmitProof(context.Background(), model.ID, ProofInput{TransactionID: "999999999", Image: jpegImage}); !errors.Is(err, ErrProofNotAllowed) {
		t.Fatalf("proof after success: %v", err)
	}
}

func TestNotFoundAndInvalidIDs(t *testing.T) {
	r, _ := newRepo(t)
	missing := utils.NewULID()
	if _, err := r.GetPayment(context.Background(), missing); !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf("get: %v", err)
	}
	if _, _, err := r.Transition(context.Background(), missing, StateFailed, Patch{}, EventInput{Source: EventSourceSystem, Kind: "x"}); !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf("transition: %v", err)
	}
	if _, err := r.Update(context.Background(), missing, Patch{}, EventInput{Source: EventSourceSystem, Kind: "x"}); !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf("update: %v", err)
	}
	if _, err := r.SubmitProof(context.Background(), missing, ProofInput{TransactionID: "123456", Image: jpegImage}); !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf("proof: %v", err)
	}
	if _, _, err := r.GetProofImage(context.Background(), missing); !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf("image: %v", err)
	}
	for _, call := range []func() error{
		func() error { _, err := r.GetPayment(context.Background(), "nope"); return err },
		func() error {
			_, _, err := r.Transition(context.Background(), "nope", StateFailed, Patch{}, EventInput{})
			return err
		},
		func() error { _, err := r.Update(context.Background(), "nope", Patch{}, EventInput{}); return err },
		func() error { _, err := r.SubmitProof(context.Background(), "nope", ProofInput{}); return err },
		func() error { _, _, err := r.GetProofImage(context.Background(), "nope"); return err },
	} {
		if err := call(); !errors.Is(err, ErrPaymentIDInvalid) {
			t.Fatalf("invalid id: %v", err)
		}
	}
	if _, _, err := r.Transition(context.Background(), missing, "DONE", Patch{}, EventInput{Source: EventSourceSystem, Kind: "x"}); !errors.Is(err, ErrStateInvalid) {
		t.Fatalf("bad state: %v", err)
	}
	if _, err := r.GetPaymentByReference(context.Background(), "X", "ref"); !errors.Is(err, ErrProviderInvalid) {
		t.Fatalf("bad provider: %v", err)
	}
	if _, err := r.GetPaymentByReference(context.Background(), ProviderInstamojo, ""); !errors.Is(err, ErrReferenceInvalid) {
		t.Fatalf("bad reference: %v", err)
	}
	if _, err := r.ListOrderPayments(context.Background(), ""); !errors.Is(err, ErrOrderIDInvalid) {
		t.Fatalf("bad order: %v", err)
	}
	list, err := r.ListOrderPayments(context.Background(), "nothing")
	if err != nil || list == nil || len(list) != 0 {
		t.Fatalf("empty list: %v %v", err, list)
	}
}

func TestContextChecks(t *testing.T) {
	r, _ := newRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	id := utils.NewULID()
	calls := []func() error{
		func() error { _, _, err := r.CreatePayment(ctx, createInput("o")); return err },
		func() error { _, err := r.GetPayment(ctx, id); return err },
		func() error { _, err := r.GetPaymentByReference(ctx, ProviderInstamojo, "r"); return err },
		func() error { _, err := r.ListOrderPayments(ctx, "o"); return err },
		func() error { _, _, err := r.Transition(ctx, id, StateFailed, Patch{}, EventInput{}); return err },
		func() error { _, err := r.Update(ctx, id, Patch{}, EventInput{}); return err },
		func() error { _, err := r.SubmitProof(ctx, id, ProofInput{}); return err },
		func() error { _, _, err := r.GetProofImage(ctx, id); return err },
	}
	for i, call := range calls {
		if err := call(); !errors.Is(err, ErrCtxInvalid) {
			t.Fatalf("call %d: %v", i, err)
		}
	}
}

func TestSchemaBackstop(t *testing.T) {
	r, _ := newRepo(t)
	if _, err := r.writer.Exec(`INSERT INTO payments (id, order_id, user_id, provider, state, amount, currency, purpose, metadata, created_at, updated_at) VALUES ('01ARZ3NDEKTSV4RRFFQ69G5FAV', 'o', 'u', 'INSTAMOJO', 'CREATED', 0, 'INR', '', '{}', 1, 1)`); err == nil {
		t.Fatalf("amount 0 must be rejected by the schema")
	}
	if _, err := r.writer.Exec(`INSERT INTO payments (id, order_id, user_id, provider, state, amount, currency, purpose, metadata, created_at, updated_at) VALUES ('01ARZ3NDEKTSV4RRFFQ69G5FAV', 'o', 'u', 'PAYPAL', 'CREATED', 1, 'INR', '', '{}', 1, 1)`); err == nil {
		t.Fatalf("provider must be checked by the schema")
	}
	a := mustCreate(t, r, createInput("dup-order"))
	if _, err := r.writer.Exec(`INSERT INTO payments (id, order_id, user_id, provider, state, amount, currency, purpose, metadata, created_at, updated_at) VALUES ('01ARZ3NDEKTSV4RRFFQ69G5FAW', 'dup-order', 'u', 'INSTAMOJO', 'CREATED', 1, 'INR', '', '{}', 1, 1)`); err == nil {
		t.Fatalf("second open payment for %s must be rejected by the partial unique index", a.OrderID)
	}
}

func TestReopenKeepsData(t *testing.T) {
	dir := t.TempDir()
	repo, err := NewRepository([]string{dir, "payments.db"}, 0o600)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	model := mustCreate(t, repo, createInput("order-1"))
	if err := repo.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	reopened, err := NewRepository([]string{dir, "payments.db"}, 0o600)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()
	got, err := reopened.GetPayment(context.Background(), model.ID)
	if err != nil || got.OrderID != "order-1" {
		t.Fatalf("after reopen: %v", err)
	}
	if _, err := NewRepository([]string{filepath.Join(dir, "missing-dir"), "x.db"}, 0o600); err == nil {
		t.Fatalf("missing directory must fail")
	}
}

func TestStoredDataCorrupted(t *testing.T) {
	r, _ := newRepo(t)
	model := mustCreate(t, r, createInput("order-1"))
	if _, err := r.writer.Exec(`UPDATE payments SET metadata = 'not json' WHERE id = ?`, model.ID); err != nil {
		t.Fatalf("corrupt: %v", err)
	}
	if _, err := r.GetPayment(context.Background(), model.ID); !errors.Is(err, ErrStoredDataCorrupted) {
		t.Fatalf("got %v", err)
	}
}

func TestEnums(t *testing.T) {
	if !ProviderInstamojo.IsValid() || Provider("x").IsValid() || ProviderSelfUPI.String() != "SELF_UPI" {
		t.Fatalf("provider")
	}
	if !StateVerifying.IsOpen() || StateFailed.IsOpen() || !StateExpired.IsClosed() || StateCreated.IsClosed() || !StateSucceeded.IsFinal() || StateFailed.IsFinal() || State("x").IsValid() {
		t.Fatalf("state")
	}
	if !CurrencyINR.IsValid() || Currency("USD").IsValid() || CurrencyINR.String() != "INR" || EventSourcePoll.String() != "POLL" || EventSource("x").IsValid() {
		t.Fatalf("currency/source")
	}
	if _, err := NewStateMachine("x"); !errors.Is(err, ErrStateInvalid) {
		t.Fatalf("machine: %v", err)
	}
}
