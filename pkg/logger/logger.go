package logger

import (
	"log/slog"
	"os"

	"jaga-warung-go/internal/config"
)

func InitLogger(cfg *config.Config) {
	var handler slog.Handler
	switch cfg.LogLevel {
	case slog.LevelDebug, slog.LevelInfo:
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})
	default:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})
	}
	slog.SetDefault(slog.New(handler))
}
