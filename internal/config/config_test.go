package config

import (
	"testing"
	"time"
)

// Zero TTLs make every access token expire instantly, so the defaults must
// never be zero when the env vars are absent.
func TestLoadConfigDefaultsNonZeroTTL(t *testing.T) {
	t.Setenv("ACCESS_TTL", "")
	t.Setenv("REFRESH_TTL", "")
	t.Setenv("DATABASE_URL", "")

	cfg := LoadConfig()

	if cfg.AccessTTL <= 0 {
		t.Fatalf("AccessTTL = %v, want > 0", cfg.AccessTTL)
	}
	if cfg.RefreshTTL <= 0 {
		t.Fatalf("RefreshTTL = %v, want > 0", cfg.RefreshTTL)
	}
	if cfg.AccessTTL != 15*time.Minute {
		t.Fatalf("AccessTTL = %v, want 15m", cfg.AccessTTL)
	}
	if cfg.DatabaseURL == "" {
		t.Fatal("DatabaseURL empty, want default DSN")
	}
}

func TestLoadConfigParsesTTL(t *testing.T) {
	t.Setenv("ACCESS_TTL", "30m")
	t.Setenv("REFRESH_TTL", "48h")

	cfg := LoadConfig()

	if cfg.AccessTTL != 30*time.Minute {
		t.Fatalf("AccessTTL = %v, want 30m", cfg.AccessTTL)
	}
	if cfg.RefreshTTL != 48*time.Hour {
		t.Fatalf("RefreshTTL = %v, want 48h", cfg.RefreshTTL)
	}
}

func TestLoadConfigRejectsMalformedTTL(t *testing.T) {
	t.Setenv("ACCESS_TTL", "not-a-duration")

	cfg := LoadConfig()

	if cfg.AccessTTL != 15*time.Minute {
		t.Fatalf("AccessTTL = %v, want fallback 15m", cfg.AccessTTL)
	}
}

func TestLoadConfigParsesKafkaBrokers(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", " a:9092 , b:9094 ,, ")

	cfg := LoadConfig()

	if len(cfg.KafkaBrokers) != 2 {
		t.Fatalf("KafkaBrokers = %v, want 2 entries", cfg.KafkaBrokers)
	}
	if cfg.KafkaBrokers[0] != "a:9092" || cfg.KafkaBrokers[1] != "b:9094" {
		t.Fatalf("KafkaBrokers = %v, want [a:9092 b:9094]", cfg.KafkaBrokers)
	}
}

func TestLoadConfigKafkaBrokersEmptyMeansDisabled(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "")

	if brokers := LoadConfig().KafkaBrokers; len(brokers) != 0 {
		t.Fatalf("KafkaBrokers = %v, want empty (publisher disabled)", brokers)
	}
}

func TestLoadConfigKafkaGroupIDDefault(t *testing.T) {
	t.Setenv("KAFKA_GROUP_ID", "")

	if got := LoadConfig().KafkaGroupID; got != "jaga-warung-worker" {
		t.Fatalf("KafkaGroupID = %q, want default jaga-warung-worker", got)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"complete", Config{JWTSecret: "s", AdminPhone: "p", AdminPassword: "pw"}, false},
		{"missing secret", Config{AdminPhone: "p", AdminPassword: "pw"}, true},
		{"missing phone", Config{JWTSecret: "s", AdminPassword: "pw"}, true},
		{"missing password", Config{JWTSecret: "s", AdminPhone: "p"}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
