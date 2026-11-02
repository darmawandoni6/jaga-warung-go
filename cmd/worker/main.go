package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"jaga-warung-go/internal/config"
	deliverykafka "jaga-warung-go/internal/delivery/kafka"
	"jaga-warung-go/pkg/kafka"
	"jaga-warung-go/pkg/logger"
)

// Worker is the Kafka consumer entrypoint. Log-only demo: it needs no database
// and no API secrets, so config.Validate (JWT/ADMIN) is skipped on purpose.
func main() {
	cfg := config.LoadConfig()
	logger.InitLogger(cfg)

	if len(cfg.KafkaBrokers) == 0 {
		log.Fatal("KAFKA_BROKERS is required for the worker")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	consumer := kafka.NewConsumer(cfg.KafkaBrokers, cfg.KafkaGroupID)
	defer consumer.Close()

	slog.Info("kafka worker berjalan", "group", cfg.KafkaGroupID, "brokers", cfg.KafkaBrokers)
	deliverykafka.Run(ctx, consumer, deliverykafka.Handle)
	slog.Info("kafka worker berhenti dengan aman")
}
