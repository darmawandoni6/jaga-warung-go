package kafka

import (
	"context"
	"encoding/json"
	"testing"

	"jaga-warung-go/internal/domain"
)

func TestPublishDisabledIsNoop(t *testing.T) {
	p := NewProducer(nil)
	if p.writer != nil {
		t.Fatal("expected a disabled producer with no writer")
	}

	// Both the zero producer and a nil pointer must be safe: usecases receive
	// nil in tests and a disabled producer when KAFKA_BROKERS is unset.
	p.Publish(context.Background(), TopicEvents, "abc", nil)
	var nilPub *Producer
	nilPub.Publish(context.Background(), TopicEvents, "abc", nil)

	if err := p.Close(); err != nil {
		t.Fatalf("Close() = %v, want nil", err)
	}
}

func TestNewMessageSetsTopicKeyAndEnvelope(t *testing.T) {
	msg, err := newMessage(TopicEvents, "id-1", domain.NewEvent("transaction.created", "transaction", "id-1", map[string]string{"a": "b"}))
	if err != nil {
		t.Fatalf("newMessage() error = %v", err)
	}
	if msg.Topic != TopicEvents {
		t.Fatalf("Topic = %q, want %q", msg.Topic, TopicEvents)
	}
	if string(msg.Key) != "id-1" {
		t.Fatalf("Key = %q, want %q", msg.Key, "id-1")
	}

	var got map[string]any
	if err := json.Unmarshal(msg.Value, &got); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	for _, field := range []string{"event_id", "type", "entity", "id", "at", "data"} {
		if _, ok := got[field]; !ok {
			t.Fatalf("envelope missing %q: %s", field, msg.Value)
		}
	}
	if got["type"] != "transaction.created" || got["entity"] != "transaction" || got["id"] != "id-1" {
		t.Fatalf("unexpected envelope: %s", msg.Value)
	}
}

func TestNewMessageOmitsDataForDelete(t *testing.T) {
	msg, err := newMessage(TopicEvents, "id-1", domain.NewEvent("transaction.deleted", "transaction", "id-1", nil))
	if err != nil {
		t.Fatalf("newMessage() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(msg.Value, &got); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if _, ok := got["data"]; ok {
		t.Fatalf("data must be absent for id-only events: %s", msg.Value)
	}
}
