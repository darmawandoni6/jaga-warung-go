package kafka

import (
	"context"
	"encoding/json"
	"log/slog"

	kafkago "github.com/segmentio/kafka-go"

	"jaga-warung-go/internal/domain"
	"jaga-warung-go/pkg/logger"
)

// Handle decodes one event and logs it. Log-only: no DB writes, no side
// effects. Typed payloads are decoded per event type, so a schema drift shows
// up in the worker log instead of silently passing through.
func Handle(_ context.Context, msg kafkago.Message) {
	var ev domain.Event
	if err := json.Unmarshal(msg.Value, &ev); err != nil {
		slog.Error("kafka consumer: decode envelope", "err", err, "partition", msg.Partition, "offset", msg.Offset)
		return
	}

	switch ev.Type {
	case "transaction.created", "transaction.updated":
		var row domain.TransactionEvent
		if decode(ev, &row) {

			logger.Log(map[string]interface{}{
				"message": "kafka consumer: transaction created",
				"data":    row,
			})
			slog.Info("event", "type", ev.Type, "event_id", ev.EventID, "id", ev.ID,
				"total", row.TotalAmount, "items", len(row.Items))
		}
	case "transaction.item.created", "transaction.item.updated":
		var row domain.TransactionItemEvent
		if decode(ev, &row) {
			logger.Log(map[string]interface{}{
				"message": "kafka consumer: transaction item created",
				"data":    row,
			})
			slog.Info("event", "type", ev.Type, "event_id", ev.EventID, "id", ev.ID,
				"barcode", row.ProductBarcode, "qty", row.Quantity)
		}
	case "stock.movement.created", "stock.movement.updated":
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
		slog.Info("event", "type", ev.Type, "event_id", ev.EventID, "id", ev.ID, "key", string(msg.Key))
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
