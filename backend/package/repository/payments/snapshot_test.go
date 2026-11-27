package paymentsRepository

import (
	"context"
	"strings"
	"testing"
)

func TestDump_LeavesOutProofImage(t *testing.T) {
	r, _ := newRepo(t)
	model := mustCreate(t, r, createInput("order-1"))
	if _, _, err := r.Transition(context.Background(), model.ID, StateAwaitingPayment, Patch{}, EventInput{Source: EventSourceSystem, Kind: "await"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.SubmitProof(context.Background(), model.ID, ProofInput{TransactionID: "412345678901", Image: jpegImage}); err != nil {
		t.Fatal(err)
	}
	tables, err := r.Dump(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 2 || tables[0].Name != "payments" || tables[1].Name != "payment_events" {
		t.Fatalf("tables = %+v", tables)
	}
	if len(tables[0].Rows) != 1 || tables[0].Rows[0].Key != model.ID {
		t.Fatalf("payments rows = %+v", tables[0].Rows)
	}
	row := tables[0].Rows[0].JSON
	if strings.Contains(row, `"proof_image"`) || !strings.Contains(row, `"proof_image_mime":"image/jpeg"`) || !strings.Contains(row, "412345678901") {
		t.Fatalf("payment row = %s", row)
	}
	if len(tables[1].Rows) < 2 {
		t.Fatalf("events = %+v", tables[1].Rows)
	}
}
