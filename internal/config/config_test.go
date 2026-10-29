package config

import (
	"log/slog"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	t.Setenv("JWT_SECRET", "test")
	t.Setenv("ACCESS_TTL", "1m")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AccessTTL != time.Minute {
		t.Errorf("AccessTTL = %v, want 1m", cfg.AccessTTL)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want debug", cfg.LogLevel)
	}
}

func TestLoadRequiresJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when JWT_SECRET is empty")
	}
}
