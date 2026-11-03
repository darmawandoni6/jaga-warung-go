// Package kafka is the low-level Kafka transport: it sends and receives raw
// messages. Event schemas live in internal/domain.
package kafka

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

// TopicEvents is the single topic every domain event lands on.
const TopicEvents = "jaga-warung.events"

// TopicDLQ collects events the worker could not apply. It is a dead end, not a
// retry queue: replay it by hand once the cause is fixed.
const TopicDLQ = "jaga-warung.events.dlq"

// Producer writes events. An empty broker list returns a disabled no-op so
// KAFKA_BROKERS can stay unset.
type Producer struct {
	writer *kafkago.Writer
}

func NewProducer(brokers []string) *Producer {
	if len(brokers) == 0 {
		slog.Info("kafka producer disabled: KAFKA_BROKERS is empty")
		return &Producer{}
	}
	return &Producer{writer: &kafkago.Writer{
		Addr:                   kafkago.TCP(brokers...),
		Balancer:               &kafkago.Hash{},
		WriteTimeout:           2 * time.Second,
		AllowAutoTopicCreation: true,
		Async:                  true,
	}}
}

// Publish sends one message synchronously, keyed by the caller. Failures are
// logged and swallowed: the API write that triggered the event already
// committed and must still succeed.
// ponytail: at-most-once; upgrade to a transactional outbox when losing an
// event matters.
func (p *Producer) Publish(ctx context.Context, topic, key string, v any) {
	if p == nil || p.writer == nil {
		return
	}
	msg, err := newMessage(topic, key, v)
	if err != nil {
		slog.Error("kafka: marshal event", "topic", topic, "err", err)
		return
	}
	// Debug-level: the full wire payload, so the publish side of the flow is
	// visible without a broker UI. Enable with LOG_LEVEL=debug.
	slog.Debug("kafka: publish", "topic", topic, "key", key, "event", string(msg.Value))
	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		slog.Error("kafka: publish failed", "topic", topic, "key", key, "err", err)
	}
}

// newMessage builds the wire message; separate so tests can assert topic, key
// and body without a broker.
func newMessage(topic, key string, v any) (kafkago.Message, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return kafkago.Message{}, err
	}
	return kafkago.Message{Topic: topic, Key: []byte(key), Value: body}, nil
}

// Close releases the writer. Safe on a disabled producer.
func (p *Producer) Close() error {
	if p == nil || p.writer == nil {
		return nil
	}
	return p.writer.Close()
}
