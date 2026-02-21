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

func TestCriticalAlertCreatesIncident(t *testing.T) {
	ts := httptest.NewServer(setupServer(t).Router())
	defer ts.Close()
	c := newClient(ts)
	login(t, c, ts.URL)

	payload := map[string]any{"title": "db down", "description": "timeout", "severity": "critical"}
	b, _ := json.Marshal(payload)
	res, err := c.Post(ts.URL+"/api/alerts", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 got %d", res.StatusCode)
	}

	incRes, err := c.Get(ts.URL + "/api/incidents")
	if err != nil {
		t.Fatal(err)
	}
	defer incRes.Body.Close()
	var incidents []map[string]any
	if err := json.NewDecoder(incRes.Body).Decode(&incidents); err != nil {
		t.Fatal(err)
	}
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident got %d", len(incidents))
	}

	alertsRes, err := c.Get(ts.URL + "/api/alerts")
	if err != nil {
		t.Fatal(err)
	}
	defer alertsRes.Body.Close()
	var alerts []map[string]any
	if err := json.NewDecoder(alertsRes.Body).Decode(&alerts); err != nil {
		t.Fatal(err)
	}
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

	payload := map[string]any{"title": "queue lag", "description": "retry queue is increasing", "severity": "warning"}
	b, _ := json.Marshal(payload)
	res, err := c.Post(ts.URL+"/api/alerts", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 got %d", res.StatusCode)
	}

	incRes, err := c.Get(ts.URL + "/api/incidents")
	if err != nil {
		t.Fatal(err)
	}
	defer incRes.Body.Close()
	var incidents []map[string]any
	if err := json.NewDecoder(incRes.Body).Decode(&incidents); err != nil {
		t.Fatal(err)
	}
	if len(incidents) != 0 {
		t.Fatalf("expected 0 incidents got %d", len(incidents))
	}

	alertsRes, err := c.Get(ts.URL + "/api/alerts")
	if err != nil {
		t.Fatal(err)
	}
	defer alertsRes.Body.Close()
	var alerts []map[string]any
	if err := json.NewDecoder(alertsRes.Body).Decode(&alerts); err != nil {
		t.Fatal(err)
	}
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
