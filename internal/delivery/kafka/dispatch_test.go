package kafka

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"jaga-warung-go/internal/domain"
)

// Every stock-bearing type reaches the handler; log-only types do not. A typo
// in an event-type constant would break stock silently, so pin the routing.
func TestDispatcherRoutesStockEvents(t *testing.T) {
	txID := uuid.New()
	itemID := uuid.New()
	item := domain.TransactionItemEvent{ID: itemID, TransactionID: txID}

	cases := []struct {
		typ  string
		data any
	}{
		{domain.EventTypeTransactionCreated, nil},
		{domain.EventTypeTransactionUpdated, nil},
		{domain.EventTypeTransactionDeleted, nil},
		{domain.EventTypeTransactionItemCreated, item},
		{domain.EventTypeTransactionItemUpdated, item},
		{domain.EventTypeTransactionItemDeleted, item},
		{domain.EventTypeStockMovementCreated, nil},
		{domain.EventTypeStockMovementDeleted, nil},
	}

	for _, tc := range cases {
		t.Run(tc.typ, func(t *testing.T) {
			var rebuilt int
			h := &TransactionHandler{rebuild: func(context.Context, uuid.UUID) error {
				rebuilt++
				return nil
			}}
			id := txID.String()
			if tc.data != nil {
				id = itemID.String()
			}
			ev := domain.NewEvent(tc.typ, "x", id, tc.data)

			if err := NewDispatcher(h)(context.Background(), ev); err != nil {
				t.Fatalf("dispatch %s: %v", tc.typ, err)
			}

			want := 0
			if tc.typ != domain.EventTypeStockMovementCreated && tc.typ != domain.EventTypeStockMovementDeleted {
				want = 1
			}
			if rebuilt != want {
				t.Fatalf("rebuilt %d times for %s, want %d", rebuilt, tc.typ, want)
			}
		})
	}
}
