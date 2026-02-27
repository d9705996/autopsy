package auth_test

import (
	"testing"
	"time"

	"github.com/d9705996/autopsy/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-at-least-32-bytes-long"

func TestIssueAndParseAccessToken(t *testing.T) {
	token, err := auth.IssueAccessToken("user-1", "user@example.com", []string{"Viewer"}, "", testSecret, 15*time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	claims, err := auth.ParseAccessToken(token, testSecret)
	require.NoError(t, err)
	assert.Equal(t, "user-1", claims.UserID)
	assert.Equal(t, "user@example.com", claims.Email)
	assert.Equal(t, []string{"Viewer"}, claims.Roles)
}

func TestParseAccessToken_ExpiredToken(t *testing.T) {
	// Issue a token with a -1 minute TTL so it is already expired.
	token, err := auth.IssueAccessToken("user-1", "user@example.com", []string{"Admin"}, "", testSecret, -time.Minute)
	require.NoError(t, err)

	_, err = auth.ParseAccessToken(token, testSecret)
	require.Error(t, err)
}

func TestParseAccessToken_WrongSecret(t *testing.T) {
	token, err := auth.IssueAccessToken("user-1", "user@example.com", nil, "", testSecret, 15*time.Minute)
	require.NoError(t, err)

	_, err = auth.ParseAccessToken(token, "wrong-secret")
	require.Error(t, err)
}

func TestParseAccessToken_Garbage(t *testing.T) {
	_, err := auth.ParseAccessToken("not.a.jwt", testSecret)
	require.Error(t, err)
}
