package config

import (
	"log/slog"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	t.Setenv("JWT_SECRET", "test")
	t.Setenv("ADMIN_PHONE", "081234567890")
	t.Setenv("ADMIN_PASSWORD", "secret")
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
	t.Setenv("ADMIN_PHONE", "081234567890")
	t.Setenv("ADMIN_PASSWORD", "secret")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when JWT_SECRET is empty")
	}
}

func TestLoadRequiresAdminCredentials(t *testing.T) {
	t.Setenv("JWT_SECRET", "test")
	t.Setenv("ADMIN_PHONE", "")
	t.Setenv("ADMIN_PASSWORD", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when admin credentials are empty")
	}
}
