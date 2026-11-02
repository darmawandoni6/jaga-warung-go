package logger

import (
	"encoding/json"
	"fmt"
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

func Log(data interface{}) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Println("marshal err:", err)
	} else {
		fmt.Println(string(b))
	}
}
