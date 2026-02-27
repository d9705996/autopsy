// Package config loads all runtime configuration from environment variables.
// No config files and no third-party config framework are used.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration for Autopsy.
type Config struct {
	HTTP   HTTPConfig
	DB     DBConfig
	Log    LogConfig
	JWT    JWTConfig
	AI     AIConfig
	App    AppConfig
	Worker WorkerConfig
	OTel   OTelConfig
}

// HTTPConfig holds HTTP server configuration.
type HTTPConfig struct {
	Port int
}

// DBConfig holds database connection configuration.
type DBConfig struct {
	Driver   string // "sqlite" (default) or "postgres"
	DSN      string // required when Driver == "postgres"
	File     string // SQLite database file path (default: "autopsy.db")
	MaxConns int    // Postgres only
}

// LogConfig controls structured logging output.
type LogConfig struct {
	Level  string
	Format string
}

// JWTConfig holds JSON Web Token signing and expiry settings.
type JWTConfig struct {
	Secret     string //nolint:gosec // intentional: holds JWT signing secret loaded from env
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

// AIConfig holds AI provider connection settings.
type AIConfig struct {
	Provider string
	APIKey   string //nolint:gosec // intentional: holds AI provider API key loaded from env
	APIBase  string
	Model    string
}

// AppConfig holds application-level settings such as seed credentials.
type AppConfig struct {
	SeedAdminEmail    string
	SeedAdminPassword string
}

// WorkerConfig holds background worker settings.
type WorkerConfig struct {
	Concurrency int
}

// OTelConfig holds OpenTelemetry exporter settings.
type OTelConfig struct {
	OTLPEndpoint string
}

// Load reads configuration from environment variables, applies defaults,
// and returns an error if any required field is absent.
func Load() (*Config, error) {
	cfg := &Config{}

	// HTTP
	cfg.HTTP.Port = envInt("HTTP_PORT", 8080)

	// DB
	cfg.DB.Driver = envStr("DB_DRIVER", "sqlite")
	cfg.DB.File = envStr("DB_FILE", "autopsy.db")
	cfg.DB.DSN = os.Getenv("DB_DSN")
	if cfg.DB.Driver == "postgres" && cfg.DB.DSN == "" {
		return nil, errors.New("DB_DSN is required when DB_DRIVER=postgres")
	}
	cfg.DB.MaxConns = envInt("DB_MAX_CONNS", 25)

	// Log
	cfg.Log.Level = envStr("LOG_LEVEL", "info")
	cfg.Log.Format = envStr("LOG_FORMAT", "json")

	// JWT (required)
	cfg.JWT.Secret = os.Getenv("JWT_SECRET")
	if cfg.JWT.Secret == "" {
		return nil, errors.New("JWT_SECRET is required")
	}
	var err error
	cfg.JWT.AccessTTL, err = envDuration("JWT_ACCESS_TTL", 15*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("JWT_ACCESS_TTL: %w", err)
	}
	cfg.JWT.RefreshTTL, err = envDuration("JWT_REFRESH_TTL", 720*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("JWT_REFRESH_TTL: %w", err)
	}

	// AI
	cfg.AI.Provider = envStr("AI_PROVIDER", "noop")
	cfg.AI.APIKey = os.Getenv("AI_API_KEY")
	cfg.AI.APIBase = envStr("AI_API_BASE", "https://api.openai.com/v1")
	cfg.AI.Model = envStr("AI_MODEL", "gpt-4o-mini")

	// App
	cfg.App.SeedAdminEmail = envStr("SEED_ADMIN_EMAIL", "admin@autopsy.local")
	cfg.App.SeedAdminPassword = os.Getenv("SEED_ADMIN_PASSWORD")

	// Worker
	cfg.Worker.Concurrency = envInt("WORKER_CONCURRENCY", 10)

	// OTel
	cfg.OTel.OTLPEndpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	return cfg, nil
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envDuration(key string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", v, err)
	}
	return d, nil
}
