package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"jaga-warung-go/internal/domain"
	pkgkafka "jaga-warung-go/pkg/kafka"
)

// isRetryable reports whether a failure may succeed on a second attempt. The
// deny-list is deliberate: a deterministic business error or an undecodable
// payload fails identically forever, so it is dead on the first attempt.
// Everything else is treated as transient (DB lock timeout, connection reset)
// and gets the retry budget.
func isRetryable(err error) bool {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError

	switch {
	case errors.Is(err, domain.ErrInsufficientStock), errors.Is(err, domain.ErrNotFound):
		return false
	case errors.As(err, &syntaxErr), errors.As(err, &typeErr):
		return false
	}
	return true
}

// publishFailure sends a dead message to the DLQ. The caller still commits the
// offset: the message is dead, not pending. Replay is manual by design.
func publishFailure(ctx context.Context, publish func(context.Context, string, string, any), key []byte, ev domain.Event, cause error) {
	slog.Error("kafka consumer: message dead, publishing to DLQ",
		"type", ev.Type, "event_id", ev.EventID, "id", ev.ID, "err", cause)
	publish(ctx, pkgkafka.TopicDLQ, string(key), domain.NewEvent(
		domain.EventTypeEventProcessingFailed, "consumer", ev.ID, domain.ConsumerFailureEvent{
			OriginalEventID: ev.EventID,
			OriginalType:    ev.Type,
			EntityID:        ev.ID,
			OriginalData:    ev.Data,
			Error:           cause.Error(),
			At:              time.Now().UTC(),
		}))
}
