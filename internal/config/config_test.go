package config_test

import (
"os"
"testing"
"time"

"github.com/d9705996/autopsy/internal/config"
"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
)

func TestLoad_MissingDBDSN(t *testing.T) {
// DB_DSN is only required when DB_DRIVER=postgres.
t.Setenv("DB_DRIVER", "postgres")
t.Setenv("DB_DSN", "")
t.Setenv("JWT_SECRET", "test-secret")
_, err := config.Load()
require.Error(t, err)
assert.Contains(t, err.Error(), "DB_DSN")
}

func TestLoad_SQLiteNoDBDSN(t *testing.T) {
// With sqlite driver, DB_DSN is not required.
t.Setenv("DB_DRIVER", "sqlite")
t.Setenv("DB_DSN", "")
t.Setenv("JWT_SECRET", "test-secret")
_, err := config.Load()
require.NoError(t, err)
}

func TestLoad_MissingJWTSecret(t *testing.T) {
t.Setenv("DB_DSN", "postgres://localhost/test")
t.Setenv("JWT_SECRET", "")
_, err := config.Load()
require.Error(t, err)
assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestLoad_Defaults(t *testing.T) {
t.Setenv("DB_DSN", "postgres://localhost/test")
t.Setenv("JWT_SECRET", "test-secret")
// Clear optional vars to ensure defaults apply
os.Unsetenv("HTTP_PORT")
os.Unsetenv("LOG_LEVEL")
os.Unsetenv("LOG_FORMAT")
os.Unsetenv("AI_PROVIDER")
os.Unsetenv("WORKER_CONCURRENCY")
os.Unsetenv("DB_DRIVER")
os.Unsetenv("DB_FILE")

cfg, err := config.Load()
require.NoError(t, err)

assert.Equal(t, 8080, cfg.HTTP.Port)
assert.Equal(t, "info", cfg.Log.Level)
assert.Equal(t, "json", cfg.Log.Format)
assert.Equal(t, "noop", cfg.AI.Provider)
assert.Equal(t, 10, cfg.Worker.Concurrency)
assert.Equal(t, 15*time.Minute, cfg.JWT.AccessTTL)
assert.Equal(t, 720*time.Hour, cfg.JWT.RefreshTTL)
assert.Equal(t, "admin@autopsy.local", cfg.App.SeedAdminEmail)
assert.Equal(t, "sqlite", cfg.DB.Driver)
assert.Equal(t, "autopsy.db", cfg.DB.File)
}

func TestLoad_Overrides(t *testing.T) {
t.Setenv("DB_DSN", "postgres://localhost/test")
t.Setenv("JWT_SECRET", "test-secret")
t.Setenv("HTTP_PORT", "9090")
t.Setenv("LOG_LEVEL", "debug")
t.Setenv("LOG_FORMAT", "text")
t.Setenv("AI_PROVIDER", "openai")
t.Setenv("WORKER_CONCURRENCY", "20")
t.Setenv("JWT_ACCESS_TTL", "5m")
t.Setenv("DB_DRIVER", "sqlite")
t.Setenv("DB_FILE", "test.db")

cfg, err := config.Load()
require.NoError(t, err)

assert.Equal(t, 9090, cfg.HTTP.Port)
assert.Equal(t, "debug", cfg.Log.Level)
assert.Equal(t, "text", cfg.Log.Format)
assert.Equal(t, "openai", cfg.AI.Provider)
assert.Equal(t, 20, cfg.Worker.Concurrency)
assert.Equal(t, 5*time.Minute, cfg.JWT.AccessTTL)
assert.Equal(t, "sqlite", cfg.DB.Driver)
assert.Equal(t, "test.db", cfg.DB.File)
}

func TestLoad_InvalidDuration(t *testing.T) {
t.Setenv("DB_DSN", "postgres://localhost/test")
t.Setenv("JWT_SECRET", "test-secret")
t.Setenv("JWT_ACCESS_TTL", "not-a-duration")

_, err := config.Load()
require.Error(t, err)
assert.Contains(t, err.Error(), "JWT_ACCESS_TTL")
}
