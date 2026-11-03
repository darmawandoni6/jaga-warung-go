package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"jaga-warung-go/internal/domain"
	pkgkafka "jaga-warung-go/pkg/kafka"
)

// fakeBroker feeds a fixed list of messages, then blocks until ctx ends so Run
// only stops on cancellation.
type fakeBroker struct {
	msgs []kafkago.Message
	next int
}

func (f *fakeBroker) fetch(ctx context.Context) (kafkago.Message, error) {
	if f.next < len(f.msgs) {
		m := f.msgs[f.next]
		f.next++
		return m, nil
	}
	<-ctx.Done()
	return kafkago.Message{}, ctx.Err()
}

type recorded struct {
	topic string
	key   string
	value any
}

// loopFixture runs one message through a Loop and returns what happened.
type loopFixture struct {
	loop       *Loop
	committed  int
	dispatched int
	published  []recorded
}

func runLoop(t *testing.T, msg kafkago.Message, dispatch func(context.Context, domain.Event) error) *loopFixture {
	t.Helper()

	f := &loopFixture{}
	broker := &fakeBroker{msgs: []kafkago.Message{msg}}
	committed := make(chan struct{}, 4)
	f.loop = &Loop{
		fetch: broker.fetch,
		commit: func(context.Context, kafkago.Message) error {
			f.committed++
			committed <- struct{}{}
			return nil
		},
		dispatch: func(ctx context.Context, ev domain.Event) error {
			f.dispatched++
			return dispatch(ctx, ev)
		},
		publish: func(_ context.Context, topic, key string, v any) {
			f.published = append(f.published, recorded{topic: topic, key: key, value: v})
		},
		backoff: []time.Duration{0, 0, 0},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); f.loop.Run(ctx) }()

	// Every path commits exactly once; wait for it, then stop the loop.
	select {
	case <-committed:
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatal("loop did not process the message")
	}
	cancel()
	<-done
	return f
}

func eventMessage(t *testing.T, ev domain.Event) kafkago.Message {
	t.Helper()
	body, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	return kafkago.Message{Key: []byte(ev.ID), Value: body}
}

// A message that succeeds is dispatched once, committed once, never DLQ'd.
func TestLoopCommitsSuccess(t *testing.T) {
	ev := domain.NewEvent("transaction.created", "transaction", "tx-1", nil)
	f := runLoop(t, eventMessage(t, ev), func(context.Context, domain.Event) error { return nil })

	if f.dispatched != 1 {
		t.Fatalf("dispatched %d times, want 1", f.dispatched)
	}
	if f.committed != 1 {
		t.Fatalf("committed %d times, want 1", f.committed)
	}
	if len(f.published) != 0 {
		t.Fatalf("published %d messages, want 0", len(f.published))
	}
}

// A transient failure gets the full retry budget, then one DLQ message, and the
// offset is still committed so the partition moves on.
func TestLoopRetriesThenDLQ(t *testing.T) {
	ev := domain.NewEvent("transaction.created", "transaction", "tx-2", nil)
	f := runLoop(t, eventMessage(t, ev), func(context.Context, domain.Event) error {
		return errors.New("db down")
	})

	if f.dispatched != maxRetries+1 {
		t.Fatalf("dispatched %d times, want %d", f.dispatched, maxRetries+1)
	}
	if len(f.published) != 1 || f.published[0].topic != pkgkafka.TopicDLQ {
		t.Fatalf("published = %+v, want one %q message", f.published, pkgkafka.TopicDLQ)
	}
	if f.committed != 1 {
		t.Fatalf("committed %d times, want 1", f.committed)
	}
}

// A deterministic business error is dead on the first attempt: no retry.
func TestLoopFatalErrorSkipsRetry(t *testing.T) {
	ev := domain.NewEvent("transaction.created", "transaction", "tx-3", nil)
	f := runLoop(t, eventMessage(t, ev), func(context.Context, domain.Event) error {
		return domain.ErrInsufficientStock
	})

	if f.dispatched != 1 {
		t.Fatalf("dispatched %d times, want 1", f.dispatched)
	}
	if len(f.published) != 1 || f.published[0].topic != pkgkafka.TopicDLQ {
		t.Fatalf("published = %+v, want one %q message", f.published, pkgkafka.TopicDLQ)
	}
	if f.committed != 1 {
		t.Fatalf("committed %d times, want 1", f.committed)
	}
}

// An undecodable envelope never reaches a handler and is DLQ'd immediately.
func TestLoopDecodeFailureGoesToDLQ(t *testing.T) {
	f := runLoop(t, kafkago.Message{Key: []byte("k"), Value: []byte("{not json")},
		func(context.Context, domain.Event) error { return nil })

	if f.dispatched != 0 {
		t.Fatalf("dispatched %d times, want 0", f.dispatched)
	}
	if len(f.published) != 1 || f.published[0].topic != pkgkafka.TopicDLQ {
		t.Fatalf("published = %+v, want one %q message", f.published, pkgkafka.TopicDLQ)
	}
	if f.committed != 1 {
		t.Fatalf("committed %d times, want 1", f.committed)
	}
}

// The DLQ envelope keeps enough of the original message to replay it by hand.
func TestLoopDLQPayload(t *testing.T) {
	ev := domain.NewEvent("transaction.created", "transaction", "tx-4", nil)
	f := runLoop(t, eventMessage(t, ev), func(context.Context, domain.Event) error {
		return domain.ErrNotFound
	})

	if len(f.published) != 1 {
		t.Fatalf("published %d messages, want 1", len(f.published))
	}
	dlq, ok := f.published[0].value.(domain.Event)
	if !ok {
		t.Fatalf("published value is %T, want domain.Event", f.published[0].value)
	}
	if dlq.Type != "event.processing.failed" {
		t.Fatalf("type = %q, want event.processing.failed", dlq.Type)
	}
	var payload domain.ConsumerFailureEvent
	if err := json.Unmarshal(dlq.Data, &payload); err != nil {
		t.Fatalf("decode DLQ payload: %v", err)
	}
	if payload.OriginalEventID != ev.EventID || payload.OriginalType != ev.Type || payload.EntityID != ev.ID {
		t.Fatalf("payload = %+v, want original identity of %+v", payload, ev)
	}
}

// The retry classification is the whole policy: pin it.
func TestIsRetryable(t *testing.T) {
	var syntax *json.SyntaxError
	_ = json.Unmarshal([]byte("{"), &syntax)

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"insufficient stock", domain.ErrInsufficientStock, false},
		{"not found", domain.ErrNotFound, false},
		{"wrapped insufficient stock", errors.Join(errors.New("rebuild"), domain.ErrInsufficientStock), false},
		{"syntax error", syntax, false},
		{"unknown", errors.New("connection reset"), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRetryable(tc.err); got != tc.want {
				t.Fatalf("isRetryable(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
