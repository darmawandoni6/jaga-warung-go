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

// Worker is the Kafka consumer entrypoint. It needs the database (it applies
// the stock side effects) but no API secrets, so config.Validate (JWT/ADMIN) is
// skipped on purpose.
func main() {
	cfg := config.LoadConfig()
	logger.InitLogger(cfg)

	if len(cfg.KafkaBrokers) == 0 {
		log.Fatal("KAFKA_BROKERS is required for the worker")
	}

	db, err := config.InitDB(cfg)
	if err != nil {
		log.Fatalf("Gagal inisialisasi database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Gagal mendapatkan sql.DB instance: %v", err)
	}
	defer sqlDB.Close()

	// Same producer the API uses, for the DLQ.
	pub := kafka.NewProducer(cfg.KafkaBrokers)
	defer pub.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	consumer := kafka.NewConsumer(cfg.KafkaBrokers, cfg.KafkaGroupID)
	defer consumer.Close()

	loop := deliverykafka.Setup(consumer, db, pub)

	slog.Info("kafka worker berjalan", "group", cfg.KafkaGroupID, "brokers", cfg.KafkaBrokers)
	loop.Run(ctx)
	slog.Info("kafka worker berhenti dengan aman")
}
