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

func createTool(t *testing.T, c *http.Client, baseURL string, payload map[string]any) string {
	t.Helper()
	body, _ := json.Marshal(payload)
	res, err := c.Post(baseURL+"/api/tools", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 got %d", res.StatusCode)
	}
	var created map[string]any
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	toolID, _ := created["id"].(string)
	if toolID == "" {
		t.Fatal("expected tool id")
	}
	return toolID
}

func updateTool(t *testing.T, c *http.Client, baseURL, toolID string, payload map[string]any) {
	t.Helper()
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPut, baseURL+"/api/tools/"+toolID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 got %d", res.StatusCode)
	}
}

func deleteToolRequest(t *testing.T, c *http.Client, baseURL, toolID string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, baseURL+"/api/tools/"+toolID, nil)
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 got %d", res.StatusCode)
	}
}

func TestToolsCRUD(t *testing.T) {
	ts := httptest.NewServer(setupServer(t).Router())
	defer ts.Close()
	c := newClient(ts)
	login(t, c, ts.URL)

	toolID := createTool(t, c, ts.URL, map[string]any{
		"name":        "Browser runner",
		"description": "Run Playwright scripts",
		"server":      "browser_tools",
		"tool":        "run_playwright_script",
		"config":      map[string]string{"timeout": "60s"},
	})

	updateTool(t, c, ts.URL, toolID, map[string]any{
		"name":        "Browser runner",
		"description": "Run playwright with screenshots",
		"server":      "browser_tools",
		"tool":        "run_playwright_script",
		"config":      map[string]string{"timeout": "90s"},
	})

	listRes, err := c.Get(ts.URL + "/api/tools")
	if err != nil {
		t.Fatal(err)
	}
	defer listRes.Body.Close()
	var tools []map[string]any
	if err := json.NewDecoder(listRes.Body).Decode(&tools); err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool got %d", len(tools))
	}

	deleteToolRequest(t, c, ts.URL, toolID)
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

func TestPublicStatusPageReflectsActiveIncident(t *testing.T) {
	ts := httptest.NewServer(setupServer(t).Router())
	defer ts.Close()
	c := newClient(ts)
	login(t, c, ts.URL)

	createAlert(t, c, ts.URL, map[string]any{
		"title":       "checkout down",
		"description": "customer checkout timeout spike",
		"severity":    "critical",
	})

	res, err := http.Get(ts.URL + "/api/statuspage")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 got %d", res.StatusCode)
	}

	var status map[string]any
	if err := json.NewDecoder(res.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if status["overallStatus"] != "major_outage" {
		t.Fatalf("expected major_outage got %#v", status["overallStatus"])
	}
	incidents, ok := status["incidents"].([]any)
	if !ok || len(incidents) != 1 {
		t.Fatalf("expected 1 public incident got %#v", status["incidents"])
	}
	services, ok := status["services"].([]any)
	if !ok || len(services) != 1 {
		t.Fatalf("expected 1 service availability entry got %#v", status["services"])
	}
	service, ok := services[0].(map[string]any)
	if !ok || service["service"] != "unknown" {
		t.Fatalf("expected unknown service got %#v", services[0])
	}
}

func TestPublicStatusPageReturnsServiceAvailabilityForPeriod(t *testing.T) {
	repo := store.NewMemoryStore()
	now := time.Now().UTC()
	start := now.Add(-2 * time.Hour)
	resolved := now.Add(-1 * time.Hour)
	if _, err := repo.CreateIncident(app.Incident{
		AlertID:       "alt-1",
		Service:       "payments",
		Title:         "payments latency",
		Severity:      app.SeverityCritical,
		Status:        "resolved",
		StatusPageURL: "/status/alt-1",
		CreatedAt:     start,
		ResolvedAt:    &resolved,
	}); err != nil {
		t.Fatal(err)
	}
	server := NewServer(repo, triage.NewHeuristicAgent(), auth.New("test-secret"), testFS)
	ts := httptest.NewServer(server.Router())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/statuspage?periodHours=3")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 got %d", res.StatusCode)
	}

	var status map[string]any
	if err := json.NewDecoder(res.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	services, ok := status["services"].([]any)
	if !ok || len(services) != 1 {
		t.Fatalf("expected 1 service availability entry got %#v", status["services"])
	}
	service := services[0].(map[string]any)
	if service["service"] != "payments" {
		t.Fatalf("expected payments service got %#v", service["service"])
	}
	availability, ok := service["availabilityPercent"].(float64)
	if !ok {
		t.Fatalf("expected numeric availability got %#v", service["availabilityPercent"])
	}
	if availability <= 60 || availability >= 70 {
		t.Fatalf("expected availability near 66.6 got %f", availability)
	}
}

func TestPublicStatusPageIncludesServicesWithoutIncidents(t *testing.T) {
	repo := store.NewMemoryStore()
	if _, err := repo.EnsureService("search"); err != nil {
		t.Fatal(err)
	}
	server := NewServer(repo, triage.NewHeuristicAgent(), auth.New("test-secret"), testFS)
	ts := httptest.NewServer(server.Router())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/statuspage")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 got %d", res.StatusCode)
	}

	var status map[string]any
	if err := json.NewDecoder(res.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	services, ok := status["services"].([]any)
	if !ok || len(services) != 1 {
		t.Fatalf("expected 1 service availability entry got %#v", status["services"])
	}
	svc := services[0].(map[string]any)
	if svc["service"] != "search" {
		t.Fatalf("expected search service got %#v", svc["service"])
	}
	if svc["availabilityPercent"].(float64) != 100 {
		t.Fatalf("expected 100 availability got %#v", svc["availabilityPercent"])
	}
}
