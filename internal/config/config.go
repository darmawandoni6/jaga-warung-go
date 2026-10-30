package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

const defaultDSN = "host=localhost user=postgres password=postgres dbname=jaga_warung port=5432 sslmode=disable TimeZone=Asia/Jakarta"

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
	isDebug       bool
}

// LoadConfig reads process env. It does NOT read a .env file — there is no
// godotenv dependency. Use `make run` (exports inline) or docker-compose
// (env_file) to supply these.
func LoadConfig() *Config {
	return &Config{
		DatabaseURL:   getEnv("DATABASE_URL", defaultDSN),
		Port:          getEnv("PORT", "8080"),
		LogLevel:      parseLevel(getEnv("LOG_LEVEL", "info")),
		JWTSecret:     getEnv("JWT_SECRET", ""),
		AccessTTL:     getDuration("ACCESS_TTL", 15*time.Minute),
		RefreshTTL:    getDuration("REFRESH_TTL", 720*time.Hour),
		AdminPhone:    getEnv("ADMIN_PHONE", ""),
		AdminPassword: getEnv("ADMIN_PASSWORD", ""),
		isDebug:       getEnv("DEBUG", "false") == "true",
	}
}

// Validate fails fast on missing secrets. Without these the server would boot
// but no login could ever succeed, and access tokens would be signed with an
// empty key.
func (c *Config) Validate() error {
	var missing []string
	if c.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if c.AdminPhone == "" {
		missing = append(missing, "ADMIN_PHONE")
	}
	if c.AdminPassword == "" {
		missing = append(missing, "ADMIN_PASSWORD")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required env: %s", strings.Join(missing, ", "))
	}
	return nil
}

// parseLevel maps LOG_LEVEL to slog.Level, defaulting to Info.
func parseLevel(s string) slog.Level {
	var l slog.Level
	if err := l.UnmarshalText([]byte(s)); err != nil {
		return slog.LevelInfo
	}
	return l
}

// getDuration parses a time.Duration env var, falling back to def when unset or
// malformed. A zero TTL would make every access token expire immediately.
func getDuration(key string, def time.Duration) time.Duration {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

// Helper function untuk membaca env dengan nilai default. An env var set to the
// empty string is treated as unset, so `DATABASE_URL= make run` still works.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
