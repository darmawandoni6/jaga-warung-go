// Package kafka is the consumer presentation layer: it owns the fetch/commit
// loop and turns raw messages into logged domain events.
package kafka

import (
	"context"
	"log/slog"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	pkgkafka "jaga-warung-go/pkg/kafka"
)

// Run consumes until ctx is cancelled. The handler runs before the offset is
// committed, so the group never advances past an event on a clean restart.
// Transient broker errors are logged and retried once per second.
func Run(ctx context.Context, c *pkgkafka.Consumer, handler func(context.Context, kafkago.Message)) {
	for {
		msg, err := c.Fetch(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("kafka consumer: fetch failed", "err", err)
			time.Sleep(time.Second)
			continue
		}
		handler(ctx, msg)
		if err := c.Commit(ctx, msg); err != nil && ctx.Err() == nil {
			slog.Error("kafka consumer: commit failed", "err", err)
		}
	}
}
