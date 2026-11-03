package kafka

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"jaga-warung-go/internal/domain"
)

// newTestTransactionHandler records what Handle does without a database.
func newTestTransactionHandler(rebuildErr error) (*TransactionHandler, *[]uuid.UUID) {
	var rebuilt []uuid.UUID
	h := &TransactionHandler{
		rebuild: func(_ context.Context, txID uuid.UUID) error {
			rebuilt = append(rebuilt, txID)
			return rebuildErr
		},
	}
	return h, &rebuilt
}

// A sale event rebuilds the stock of that transaction.
func TestHandleRebuildsStock(t *testing.T) {
	h, rebuilt := newTestTransactionHandler(nil)
	txID := uuid.New()
	ev := domain.NewEvent("transaction.created", "transaction", txID.String(), nil)

	if err := h.Handle(context.Background(), ev); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if len(*rebuilt) != 1 || (*rebuilt)[0] != txID {
		t.Fatalf("rebuilt = %v, want [%s]", *rebuilt, txID)
	}
}

// An item event rebuilds the stock of its parent sale.
func TestHandleItemEventUsesParentSale(t *testing.T) {
	h, rebuilt := newTestTransactionHandler(nil)
	txID := uuid.New()
	itemID := uuid.New()
	ev := domain.NewEvent("transaction.item.updated", "transaction.item", itemID.String(),
		domain.TransactionItemEvent{ID: itemID, TransactionID: txID})

	if err := h.Handle(context.Background(), ev); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if len(*rebuilt) != 1 || (*rebuilt)[0] != txID {
		t.Fatalf("rebuilt = %v, want [%s]", *rebuilt, txID)
	}
}

// A deleted line is not id-only: it carries the parent sale, because the worker
// has no other way to know which product to rebuild.
func TestHandleItemDeletedUsesParentSale(t *testing.T) {
	h, rebuilt := newTestTransactionHandler(nil)
	txID := uuid.New()
	itemID := uuid.New()
	ev := domain.NewEvent("transaction.item.deleted", "transaction.item", itemID.String(),
		domain.TransactionItemEvent{ID: itemID, TransactionID: txID, ProductBarcode: "899"})

	if err := h.Handle(context.Background(), ev); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if len(*rebuilt) != 1 || (*rebuilt)[0] != txID {
		t.Fatalf("rebuilt = %v, want [%s]", *rebuilt, txID)
	}
}

// The handler returns the error untouched: retry/DLQ is the loop's call.
func TestHandleReturnsErrorAsIs(t *testing.T) {
	want := errors.New("boom")
	h, _ := newTestTransactionHandler(want)
	ev := domain.NewEvent("transaction.created", "transaction", uuid.NewString(), nil)

	if err := h.Handle(context.Background(), ev); !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

// Events with no stock side effect are a no-op, not a failure.
func TestHandleIgnoresOtherEvents(t *testing.T) {
	h, rebuilt := newTestTransactionHandler(nil)
	ev := domain.NewEvent("stock.movement.created", "stock.movement", uuid.NewString(), nil)

	if err := h.Handle(context.Background(), ev); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(*rebuilt) != 0 {
		t.Fatalf("rebuilt %d products, want 0", len(*rebuilt))
	}
}
