package kafka

import (
	"context"
	"encoding/json"
	"log/slog"

	"jaga-warung-go/internal/domain"
	"jaga-warung-go/pkg/logger"
)

// Dispatcher routes one decoded event to the handler that applies its side
// effect. Handlers return an error and leave retry/DLQ to the loop, so adding a
// consumer is one case here plus one wiring line in setup.go.
type Dispatcher func(ctx context.Context, ev domain.Event) error

// NewDispatcher builds the routing table.
func NewDispatcher(tx *TransactionHandler) Dispatcher {
	return func(ctx context.Context, ev domain.Event) error {
		switch ev.Type {
		case domain.EventTypeTransactionCreated, domain.EventTypeTransactionUpdated, domain.EventTypeTransactionDeleted,
			domain.EventTypeTransactionItemCreated, domain.EventTypeTransactionItemUpdated, domain.EventTypeTransactionItemDeleted:
			return tx.Handle(ctx, ev)
		default:
			// Events with no worker side effect (stock.movement.*) and unknown
			// types stay visible in the log instead of failing.
			logEvent(ev)
			return nil
		}
	}
}

// logEvent prints the events the worker has nothing to do with. Typed payloads
// are decoded per type, so schema drift shows up here instead of passing
// silently.
func logEvent(ev domain.Event) {
	switch ev.Type {
	case domain.EventTypeStockMovementCreated, domain.EventTypeStockMovementUpdated:
		var row domain.StockMovementEvent
		if decode(ev, &row) {
			logger.Log(map[string]interface{}{
				"message": "kafka consumer: stock movement created",
				"data":    row,
			})
			slog.Info("event", "type", ev.Type, "event_id", ev.EventID, "id", ev.ID,
				"barcode", row.ProductBarcode, "qty", row.Quantity,
				"previous", row.PreviousStock, "final", row.FinalStock)
		}
	default:
		// .deleted events carry no data, and unknown types stay visible.
		logger.Log(map[string]interface{}{
			"message": "kafka consumer: unknown event type",
			"data":    ev,
		})
		slog.Info("event", "type", ev.Type, "event_id", ev.EventID, "id", ev.ID)
	}
}

// decode unmarshals ev.Data into v, logging a failure.
func decode(ev domain.Event, v any) bool {
	if err := json.Unmarshal(ev.Data, v); err != nil {
		slog.Error("kafka consumer: decode payload", "type", ev.Type, "err", err)
		return false
	}
	return true
}
