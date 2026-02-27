package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/d9705996/autopsy/internal/api/middleware"
	"github.com/d9705996/autopsy/internal/auth"
	"github.com/stretchr/testify/assert"
)

const secret = "test-secret-at-least-32-bytes!!!"

func issueToken(t *testing.T, roles []string) string {
	t.Helper()
	tok, err := auth.IssueAccessToken("user-1", "u@example.com", roles, "", secret, 15*time.Minute)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return tok
}

func TestRequireAuth_MissingHeader(t *testing.T) {
	handler := middleware.RequireAuth(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireAuth_ValidToken(t *testing.T) {
	handler := middleware.RequireAuth(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.ClaimsFromContext(r.Context())
		assert.NotNil(t, claims)
		assert.Equal(t, "user-1", claims.UserID)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, []string{"Viewer"}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireAuth_InvalidToken(t *testing.T) {
	handler := middleware.RequireAuth(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer this.is.garbage")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequirePermission_Viewer_CannotCreate(t *testing.T) {
	chain := middleware.RequireAuth(secret)(
		middleware.RequirePermission("incident:create")(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusCreated)
			}),
		),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, []string{"Viewer"}))
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequirePermission_Responder_CanCreate(t *testing.T) {
	chain := middleware.RequireAuth(secret)(
		middleware.RequirePermission("incident:create")(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusCreated)
			}),
		),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, []string{"Responder"}))
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestRequirePermission_Admin_Wildcard(t *testing.T) {
	chain := middleware.RequireAuth(secret)(
		middleware.RequirePermission("anything:at:all")(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, []string{"Admin"}))
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
