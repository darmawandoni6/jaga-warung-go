// Package kafka is the consumer presentation layer: it owns the fetch/handle/
// commit loop, the failure policy and the DLQ. Handlers decode their own
// payload and return an error; they never touch the broker or the offset.
//
// consumer.go is the loop and its policy; dispatch.go routes an event type to a
// handler; dlq.go publishes dead messages; setup.go is the worker's composition
// root; transaction_handler.go applies the stock side effects.
package kafka

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"jaga-warung-go/internal/domain"
)

// Retry budget for a transient failure. One topic, one partition: a retry
// stalls the whole stream, so the total wait stays under two seconds.
const maxRetries = 3

var retryBackoff = []time.Duration{100 * time.Millisecond, 500 * time.Millisecond, time.Second}

// Loop is the generic consumer: fetch → dispatch → policy → commit. The
// dependencies are plain funcs, not interfaces, so a test can drive Run without
// a broker.
type Loop struct {
	fetch    func(ctx context.Context) (kafkago.Message, error)
	commit   func(ctx context.Context, msg kafkago.Message) error
	dispatch func(ctx context.Context, ev domain.Event) error
	publish  func(ctx context.Context, topic, key string, v any)

	// backoff defaults to retryBackoff; tests set zero durations so the policy
	// is exercised without sleeping.
	backoff []time.Duration
}

// Run consumes until ctx is cancelled. The message is committed after the
// handler, so a crash replays it; a dead message is DLQ'd and committed, so it
// never blocks the partition. Transient fetch errors are logged and retried
// once per second.
func (l *Loop) Run(ctx context.Context) {
	for {
		msg, err := l.fetch(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("kafka consumer: fetch failed", "err", err)
			if !sleep(ctx, time.Second) {
				return
			}
			continue
		}

		if !l.handle(ctx, msg) {
			// Shutdown during a retry: leave the offset uncommitted so the
			// message is redelivered on the next boot.
			return
		}
		if err := l.commit(ctx, msg); err != nil && ctx.Err() == nil {
			slog.Error("kafka consumer: commit failed", "err", err)
		}
	}
}

// handle applies the failure policy. It returns false only when ctx ended
// mid-retry, which tells Run to leave the message uncommitted.
func (l *Loop) handle(ctx context.Context, msg kafkago.Message) bool {
	var ev domain.Event
	if err := json.Unmarshal(msg.Value, &ev); err != nil {
		// An undecodable envelope can never succeed: dead on the first attempt.
		publishFailure(ctx, l.publish, msg.Key, domain.Event{ID: string(msg.Key)}, err)
		return true
	}

	var err error
	for attempt := 0; ; attempt++ {
		if err = l.dispatch(ctx, ev); err == nil {
			return true
		}
		if !isRetryable(err) || attempt == maxRetries {
			break
		}
		slog.Warn("kafka consumer: retrying", "type", ev.Type, "event_id", ev.EventID,
			"attempt", attempt+1, "err", err)
		if !sleep(ctx, l.backoffFor(attempt)) {
			return false
		}
	}

	publishFailure(ctx, l.publish, msg.Key, ev, err)
	return true
}

// backoffFor clamps to the last entry so a shorter slice in a test is safe.
func (l *Loop) backoffFor(attempt int) time.Duration {
	if len(l.backoff) == 0 {
		return 0
	}
	if attempt >= len(l.backoff) {
		return l.backoff[len(l.backoff)-1]
	}
	return l.backoff[attempt]
}

// sleep waits d or until ctx ends, reporting whether the wait completed.
func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
