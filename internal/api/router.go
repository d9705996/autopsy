// Package api wires all API routes onto the provided ServeMux.
package api

import (
"net/http"

"github.com/d9705996/autopsy/internal/api/handler"
"github.com/d9705996/autopsy/internal/api/middleware"
"github.com/d9705996/autopsy/internal/health"
)

// RegisterRoutes registers all application routes on mux.
func RegisterRoutes(mux *http.ServeMux, h *health.Handler, auth *handler.AuthHandler, jwtSecret string) {
// Public health endpoints (no auth required)
mux.HandleFunc("GET /api/v1/health", h.ServeHealth)
mux.HandleFunc("GET /api/v1/ready", h.ServeReady)

// Auth endpoints (no auth required)
mux.HandleFunc("POST /api/v1/auth/login", auth.Login)
mux.HandleFunc("POST /api/v1/auth/refresh", auth.Refresh)

// Auth-required routes — wrap with RequireAuth middleware.
protected := middleware.RequireAuth(jwtSecret)
mux.Handle("POST /api/v1/auth/logout", protected(http.HandlerFunc(auth.Logout)))

// Catch-all 404
mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
http.NotFound(w, r)
})
}
