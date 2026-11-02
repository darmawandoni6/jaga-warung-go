package kafka

import (
	"context"

	kafkago "github.com/segmentio/kafka-go"
)

// Consumer reads TopicEvents as one consumer group. Transport only: the caller
// owns the fetch/commit loop (internal/delivery/kafka) and the handler.
type Consumer struct {
	reader *kafkago.Reader
}

func NewConsumer(brokers []string, groupID string) *Consumer {
	return &Consumer{reader: kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:     brokers,
		GroupID:     groupID,
		GroupTopics: []string{TopicEvents},
		// FirstOffset: a fresh group replays the topic, useful for the demo and
		// for consumers that must not miss history.
		StartOffset: kafkago.FirstOffset,
		MinBytes:    1,
		MaxBytes:    10 << 20,
	})}
}

// Fetch returns the next message, blocking until one is available or ctx ends.
func (c *Consumer) Fetch(ctx context.Context) (kafkago.Message, error) {
	return c.reader.FetchMessage(ctx)
}

// Commit marks a message as processed for the group.
func (c *Consumer) Commit(ctx context.Context, msg kafkago.Message) error {
	return c.reader.CommitMessages(ctx, msg)
}

// Close releases the reader. Safe on a nil consumer.
func (c *Consumer) Close() error {
	if c == nil || c.reader == nil {
		return nil
	}
	return c.reader.Close()
}
