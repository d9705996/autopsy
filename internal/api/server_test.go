package api

import (
	"bytes"
	"embed"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/example/autopsy/internal/app"
	"github.com/example/autopsy/internal/auth"
	"github.com/example/autopsy/internal/store"
	"github.com/example/autopsy/internal/triage"
	_ "modernc.org/sqlite"
)

//go:embed testdata/*
var testFS embed.FS

func setupServer(t *testing.T) *Server {
	t.Helper()
	repo, err := store.NewSQLStore("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.EnsureRole(app.Role{
		ID:          0,
		Name:        "viewer",
		Description: "",
		Permissions: []string{"read:dashboard"},
		CreatedAt:   time.Time{},
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.EnsureAdminUser("admin", "admin"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return NewServer(repo, triage.NewHeuristicAgent(), auth.New("test-secret"), testFS)
}

func newClient(ts *httptest.Server) *http.Client {
	jar, _ := cookiejar.New(nil)
	c := ts.Client()
	c.Jar = jar
	return c
}

func login(t *testing.T, c *http.Client, url string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "admin"})
	res, err := c.Post(url+"/api/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("got %d", res.StatusCode)
	}
}

func createAlert(t *testing.T, c *http.Client, baseURL string, payload map[string]any) {
	t.Helper()
	b, _ := json.Marshal(payload)
	res, err := c.Post(baseURL+"/api/alerts", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 got %d", res.StatusCode)
	}
}

func fetchIncidents(t *testing.T, c *http.Client, baseURL string) []map[string]any {
	t.Helper()
	incRes, err := c.Get(baseURL + "/api/incidents")
	if err != nil {
		t.Fatal(err)
	}
	defer incRes.Body.Close()
	var incidents []map[string]any
	if err := json.NewDecoder(incRes.Body).Decode(&incidents); err != nil {
		t.Fatal(err)
	}
	return incidents
}

func fetchAlerts(t *testing.T, c *http.Client, baseURL string) []map[string]any {
	t.Helper()
	alertsRes, err := c.Get(baseURL + "/api/alerts")
	if err != nil {
		t.Fatal(err)
	}
	defer alertsRes.Body.Close()
	var alerts []map[string]any
	if err := json.NewDecoder(alertsRes.Body).Decode(&alerts); err != nil {
		t.Fatal(err)
	}
	return alerts
}

func TestCriticalAlertCreatesIncident(t *testing.T) {
	ts := httptest.NewServer(setupServer(t).Router())
	defer ts.Close()
	c := newClient(ts)
	login(t, c, ts.URL)

	createAlert(t, c, ts.URL, map[string]any{"title": "db down", "description": "timeout", "severity": "critical"})

	incidents := fetchIncidents(t, c, ts.URL)
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident got %d", len(incidents))
	}

	alerts := fetchAlerts(t, c, ts.URL)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert got %d", len(alerts))
	}
	if alerts[0]["status"] != "incident_open" {
		t.Fatalf("expected incident_open status got %#v", alerts[0]["status"])
	}
}

func TestWarningAlertDoesNotCreateIncident(t *testing.T) {
	ts := httptest.NewServer(setupServer(t).Router())
	defer ts.Close()
	c := newClient(ts)
	login(t, c, ts.URL)

	createAlert(t, c, ts.URL, map[string]any{
		"title":       "queue lag",
		"description": "retry queue is increasing",
		"severity":    "warning",
	})

	incidents := fetchIncidents(t, c, ts.URL)
	if len(incidents) != 0 {
		t.Fatalf("expected 0 incidents got %d", len(incidents))
	}

	alerts := fetchAlerts(t, c, ts.URL)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert got %d", len(alerts))
	}
	if alerts[0]["status"] != "triaged" {
		t.Fatalf("expected triaged status got %#v", alerts[0]["status"])
	}
}

func TestUnauthorizedWithoutLogin(t *testing.T) {
	ts := httptest.NewServer(setupServer(t).Router())
	defer ts.Close()

	res, err := ts.Client().Get(ts.URL + "/api/alerts")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", res.StatusCode)
	}
}
