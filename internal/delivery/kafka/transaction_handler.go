package kafka

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"

	"jaga-warung-go/internal/domain"
)

// TransactionHandler applies the stock side effects of transaction events. The
// API commits the sale and answers immediately; this handler drives
// ProductUseCase.RebuildStock from the committed rows, so a failure here can
// never fail a sale that already happened.
//
// It returns the error as-is: the loop decides between retry and DLQ, so the
// handler owns no failure policy.
type TransactionHandler struct {
	rebuild func(ctx context.Context, txID uuid.UUID) error
}

// NewTransactionHandler takes the port, not the repository: delivery never
// reaches past the usecase.
func NewTransactionHandler(stock domain.ProductUseCase) *TransactionHandler {
	return &TransactionHandler{rebuild: stock.RebuildStock}
}

// Handle rebuilds the stock of the sale an event points at.
func (h *TransactionHandler) Handle(ctx context.Context, ev domain.Event) error {
	txID, hasStock, err := transactionID(ev)
	if err != nil {
		return err
	}
	if !hasStock {
		return nil
	}
	if err := h.rebuild(ctx, txID); err != nil {
		return err
	}
	slog.Info("stock rebuilt", "type", ev.Type, "event_id", ev.EventID, "transaction_id", txID)
	return nil
}

// transactionID maps an event onto the sale whose stock must be rebuilt.
// hasStock is false for event types with no stock side effect.
func transactionID(ev domain.Event) (txID uuid.UUID, hasStock bool, err error) {
	switch ev.Type {
	case domain.EventTypeTransactionCreated, domain.EventTypeTransactionUpdated, domain.EventTypeTransactionDeleted:
		id, err := uuid.Parse(ev.ID)
		return id, true, err
	case domain.EventTypeTransactionItemCreated, domain.EventTypeTransactionItemUpdated, domain.EventTypeTransactionItemDeleted:
		// All three carry the parent sale: the deleted event is not id-only
		// anymore because the worker needs the sale to rebuild its stock.
		var row domain.TransactionItemEvent
		if err := json.Unmarshal(ev.Data, &row); err != nil {
			return uuid.Nil, true, err
		}
		return row.TransactionID, true, nil
	}
	return uuid.Nil, false, nil
}
