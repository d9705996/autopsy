// Package health exposes the /api/v1/health and /api/v1/ready HTTP handlers.
package health

import (
	"context"
	"net/http"
	"time"

	"github.com/d9705996/autopsy/internal/api/jsonapi"
	"github.com/d9705996/autopsy/internal/version"
)

// Pinger is implemented by anything that can check a downstream dependency.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Handler holds dependencies for the health and ready endpoints.
type Handler struct {
	db        Pinger
	startTime time.Time
}

// New creates a Handler. db may be nil during startup before the pool is
// established; in that case /ready will return 503 immediately.
func New(db Pinger) *Handler {
	return &Handler{db: db, startTime: time.Now()}
}

// healthAttrs is the JSON:API attributes payload for the health response.
type healthAttrs struct {
	Status        string `json:"status"`
	Version       string `json:"version"`
	Commit        string `json:"commit"`
	BuildDate     string `json:"build_date"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

// ServeHealth handles GET /api/v1/health.
func (h *Handler) ServeHealth(w http.ResponseWriter, r *http.Request) {
	jsonapi.RenderOne(w, http.StatusOK, jsonapi.ResourceObject{
		Type: "health",
		ID:   "1",
		Attributes: healthAttrs{
			Status:        "ok",
			Version:       version.Version,
			Commit:        version.Commit,
			BuildDate:     version.Date,
			UptimeSeconds: int64(time.Since(h.startTime).Seconds()),
		},
	})
}

// ServeReady handles GET /api/v1/ready.
// Returns 200 when PostgreSQL is reachable; 503 otherwise.
func (h *Handler) ServeReady(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		jsonapi.RenderError(w, http.StatusServiceUnavailable,
			"dependency_unavailable", "Service Unavailable",
			"database connection is not initialised")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		jsonapi.RenderError(w, http.StatusServiceUnavailable,
			"dependency_unavailable", "Service Unavailable",
			"database is unreachable: "+err.Error())
		return
	}

	jsonapi.RenderOne(w, http.StatusOK, jsonapi.ResourceObject{
		Type:       "ready",
		ID:         "1",
		Attributes: map[string]string{"status": "ok"},
	})
}
