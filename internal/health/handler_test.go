package health_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/d9705996/autopsy/internal/api/jsonapi"
	"github.com/d9705996/autopsy/internal/health"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPinger is an in-test implementation of health.Pinger.
type mockPinger struct{ err error }

func (m *mockPinger) Ping(_ context.Context) error { return m.err }

func TestServeHealth_AlwaysOK(t *testing.T) {
	h := health.New(&mockPinger{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	h.ServeHealth(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/vnd.api+json", w.Header().Get("Content-Type"))

	var doc jsonapi.Document
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &doc))
	assert.NotNil(t, doc.Data)
}

func TestServeReady_DBHealthy(t *testing.T) {
	h := health.New(&mockPinger{err: nil})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil)
	w := httptest.NewRecorder()
	h.ServeReady(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServeReady_DBUnhealthy(t *testing.T) {
	h := health.New(&mockPinger{err: errors.New("connection refused")})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil)
	w := httptest.NewRecorder()
	h.ServeReady(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var doc jsonapi.ErrorDocument
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &doc))
	require.Len(t, doc.Errors, 1)
	assert.Equal(t, "dependency_unavailable", doc.Errors[0].Code)
}

func TestServeReady_NilDB(t *testing.T) {
	h := health.New(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil)
	w := httptest.NewRecorder()
	h.ServeReady(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
