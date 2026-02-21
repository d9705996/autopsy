package api

import (
	"bytes"
	"embed"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	"github.com/example/autopsy/internal/auth"
	"github.com/example/autopsy/internal/store"
	"github.com/example/autopsy/internal/triage"
)

//go:embed testdata/*
var testFS embed.FS

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
	srv := NewServer(store.NewMemoryStore(), triage.NewHeuristicAgent(), auth.New("admin", "admin"), testFS)
	ts := httptest.NewServer(srv.Router())
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
}

func TestUnauthorizedWithoutLogin(t *testing.T) {
	srv := NewServer(store.NewMemoryStore(), triage.NewHeuristicAgent(), auth.New("admin", "admin"), testFS)
	ts := httptest.NewServer(srv.Router())
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
