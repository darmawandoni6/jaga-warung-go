package config

import (
	"errors"
	"log/slog"
	"os"
	"time"
)

// Config is the process environment, loaded once at startup.
type Config struct {
	DatabaseURL   string
	Port          string
	LogLevel      slog.Level
	JWTSecret     string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	AdminPhone    string
	AdminPassword string
}

// Load reads env vars and fails fast on anything missing or malformed.
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:   env("DATABASE_URL", "host=localhost user=postgres password=postgres dbname=jaga_warung port=5432 sslmode=disable TimeZone=Asia/Jakarta"),
		Port:          env("PORT", "8080"),
		JWTSecret:     env("JWT_SECRET", ""),
		AdminPhone:    env("ADMIN_PHONE", ""),
		AdminPassword: env("ADMIN_PASSWORD", ""),
	}
	if cfg.JWTSecret == "" {
		return nil, errors.New("JWT_SECRET is required")
	}
	if cfg.AdminPhone == "" || cfg.AdminPassword == "" {
		return nil, errors.New("ADMIN_PHONE and ADMIN_PASSWORD are required")
	}

	var err error
	if cfg.LogLevel, err = parseLevel(env("LOG_LEVEL", "info")); err != nil {
		return nil, err
	}
	if cfg.AccessTTL, err = envDuration("ACCESS_TTL", 15*time.Minute); err != nil {
		return nil, err
	}
	if cfg.RefreshTTL, err = envDuration("REFRESH_TTL", 720*time.Hour); err != nil {
		return nil, err
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	return time.ParseDuration(v)
}

func parseLevel(s string) (slog.Level, error) {
	var l slog.Level
	err := l.UnmarshalText([]byte(s))
	return l, err
}
