package config

import (
	"fmt"
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
	isDebug       bool
}

// LoadConfig membaca berkas .env dan mengembalikan struct Config
func LoadConfig() *Config {

	return &Config{
		DatabaseURL:   getEnv("DATABASE_URL", "host=localhost user=postgres password=postgres dbname=jaga_warung port=5432 sslmode=disable TimeZone=Asia/Jakarta"),
		Port:          getEnv("PORT", "8080"),
		JWTSecret:     getEnv("JWT_SECRET", ""),
		AdminPhone:    getEnv("ADMIN_PHONE", ""),
		AdminPassword: getEnv("ADMIN_PASSWORD", ""),
		isDebug:       getEnv("DEBUG", "false") == "true",
	}
}

// GetDSN mengembalikan Data Source Name untuk koneksi PostgreSQL
func (c *Config) GetDSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.DatabaseURL, c.Port, c.JWTSecret, c.AdminPassword, c.AdminPhone)
}

// Helper function untuk membaca env dengan nilai default
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
