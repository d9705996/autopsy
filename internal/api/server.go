package api

import (
	"embed"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/example/autopsy/internal/app"
	"github.com/example/autopsy/internal/auth"
	"github.com/example/autopsy/internal/store"
	"github.com/example/autopsy/internal/triage"
)

type Server struct {
	store store.Repository
	agent triage.Agent
	auth  *auth.Auth
	uiFS  embed.FS
}

func NewServer(st store.Repository, agent triage.Agent, authn *auth.Auth, ui embed.FS) *Server {
	return &Server{store: st, agent: agent, auth: authn, uiFS: ui}
}

func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", s.handleLogin)

	protected := http.NewServeMux()
	protected.HandleFunc("/api/alerts", s.handleAlerts)
	protected.HandleFunc("/api/incidents", s.handleIncidents)
	protected.HandleFunc("/api/postmortems", s.handlePostMortems)
	protected.HandleFunc("/api/playbooks", s.handlePlaybooks)
	protected.HandleFunc("/api/oncall", s.handleOnCall)

	mux.Handle("/api/", s.auth.Middleware(protected))
	mux.HandleFunc("/", s.handleUI)

	return mux
}

func (s *Server) handleUI(writer http.ResponseWriter, request *http.Request) {
	path := strings.TrimPrefix(request.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	content, err := s.uiFS.ReadFile("web/" + path)
	if err != nil {
		content, err = s.uiFS.ReadFile("web/index.html")
		if err != nil {
			http.Error(writer, "ui unavailable", http.StatusInternalServerError)
			return
		}
	}

	switch {
	case strings.HasSuffix(path, ".css"):
		writer.Header().Set("Content-Type", "text/css")
	case strings.HasSuffix(path, ".js"):
		writer.Header().Set("Content-Type", "application/javascript")
	}

	writer.WriteHeader(http.StatusOK)
	if _, err = writer.Write(content); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleLogin(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var loginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(request.Body).Decode(&loginRequest); err != nil {
		http.Error(writer, "invalid json", http.StatusBadRequest)
		return
	}

	if !s.auth.Login(loginRequest.Username, loginRequest.Password) {
		http.Error(writer, "invalid credentials", http.StatusUnauthorized)
		return
	}

	s.auth.SetSession(writer, loginRequest.Username)
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAlerts(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		alerts, err := s.store.Alerts()
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, http.StatusOK, alerts)
	case http.MethodPost:
		s.handleCreateAlert(writer, request)
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCreateAlert(writer http.ResponseWriter, request *http.Request) {
	var alertRequest app.Alert
	if err := json.NewDecoder(request.Body).Decode(&alertRequest); err != nil {
		http.Error(writer, "invalid json", http.StatusBadRequest)
		return
	}

	if alertRequest.Source == "" {
		alertRequest.Source = "grafana"
	}

	alert, err := s.store.SaveAlert(alertRequest)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	triageReport := s.agent.Review(alert)
	if err = s.store.UpdateAlertTriage(alert.ID, triageReport); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	alert.Triage = &triageReport
	if alert.Severity != app.SeverityCritical {
		writeJSON(writer, http.StatusCreated, map[string]any{"alert": alert})
		return
	}

	incident, err := s.store.CreateIncident(app.Incident{
		AlertID:       alert.ID,
		Title:         "Auto-created incident for critical alert: " + alert.Title,
		Severity:      alert.Severity,
		Status:        "investigating",
		StatusPageURL: "/status/" + alert.ID,
	})
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(writer, http.StatusCreated, map[string]any{"alert": alert, "incident": incident})
}

func (s *Server) handleIncidents(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	incidents, err := s.store.Incidents()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(writer, http.StatusOK, incidents)
}

func (s *Server) handlePostMortems(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		items, err := s.store.PostMortems()
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, http.StatusOK, items)
	case http.MethodPost:
		var payload app.PostMortem
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(writer, "invalid json", http.StatusBadRequest)
			return
		}
		created, err := s.store.AddPostMortem(payload)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, http.StatusCreated, created)
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handlePlaybooks(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		items, err := s.store.Playbooks()
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, http.StatusOK, items)
	case http.MethodPost:
		var payload app.Playbook
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(writer, "invalid json", http.StatusBadRequest)
			return
		}
		created, err := s.store.AddPlaybook(payload)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, http.StatusCreated, created)
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleOnCall(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		items, err := s.store.OnCall()
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, http.StatusOK, items)
	case http.MethodPost:
		var payload app.OnCallShift
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(writer, "invalid json", http.StatusBadRequest)
			return
		}
		created, err := s.store.AddShift(payload)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, http.StatusCreated, created)
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func writeJSON(writer http.ResponseWriter, status int, data any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(data); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}
