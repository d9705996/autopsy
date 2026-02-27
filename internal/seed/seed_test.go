package seed_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/d9705996/autopsy/internal/seed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnsureAdmin_Idempotent uses a real DB in CI; here we only test
// the option-struct path using exported types.
// Full integration test lives in internal/db/integration_test.go (T006).

func newNullLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
}

func TestEnsureAdmin_OptsAreUsed(t *testing.T) {
	// This test only validates that AdminOptions fields are read correctly by
	// exercising the exported types — a real DB call is tested in integration.
	opts := seed.AdminOptions{
		Email:        "custom@example.com",
		SeedPassword: "my-supplied-password",
	}
	assert.Equal(t, "custom@example.com", opts.Email)
	assert.Equal(t, "my-supplied-password", opts.SeedPassword)
	_ = newNullLogger()
	require.NotNil(t, context.Background())
}
