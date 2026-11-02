package domain

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestNewEventFillsEnvelope(t *testing.T) {
	ev := NewEvent("transaction.deleted", "transaction", "id-1", nil)

	if _, err := uuid.Parse(ev.EventID); err != nil {
		t.Fatalf("EventID = %q, want a uuid", ev.EventID)
	}
	if ev.At.IsZero() {
		t.Fatal("At is zero, want publish time")
	}
	if ev.Data != nil {
		t.Fatalf("Data = %s, want nil for id-only events", ev.Data)
	}
}

func TestNewTransactionEventRoundTripsThroughWire(t *testing.T) {
	note := "e2e"
	row := &Transaction{
		ID:            uuid.New(),
		TotalAmount:   10000,
		PaymentAmount: 20000,
		ChangeAmount:  10000,
		Note:          &note,
		Items: []TransactionItem{{
			ID:             uuid.New(),
			ProductBarcode: "899",
			ProductName:    "Teh",
			SellPrice:      5000,
			Quantity:       2,
			Subtotal:       10000,
		}},
	}

	body, err := json.Marshal(NewEvent("transaction.created", "transaction", row.ID.String(), NewTransactionEvent(row)))
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	// Consumer side: decode the envelope, then Data into the typed payload.
	var wire Event
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	var got TransactionEvent
	if err := json.Unmarshal(wire.Data, &got); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if got.ID != row.ID || got.TotalAmount != 10000 || got.Note == nil || *got.Note != "e2e" {
		t.Fatalf("unexpected payload: %+v", got)
	}
	if len(got.Items) != 1 || got.Items[0].ProductBarcode != "899" || got.Items[0].Quantity != 2 {
		t.Fatalf("unexpected items: %+v", got.Items)
	}
}

func TestNewStockMovementEventMapsAdvisoryStock(t *testing.T) {
	got := NewStockMovementEvent(&StockMovement{
		ID:             uuid.New(),
		ProductBarcode: "899",
		Type:           "restock",
		Quantity:       5,
		PreviousStock:  5,
		FinalStock:     10,
	})

	if got.PreviousStock != 5 || got.FinalStock != 10 || got.Quantity != 5 {
		t.Fatalf("unexpected payload: %+v", got)
	}
}
