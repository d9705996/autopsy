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
	store *store.MemoryStore
	agent triage.Agent
	auth  *auth.Auth
	uiFS  embed.FS
}

func NewServer(st *store.MemoryStore, agent triage.Agent, a *auth.Auth, ui embed.FS) *Server {
	return &Server{store: st, agent: agent, auth: a, uiFS: ui}
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

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	b, err := s.uiFS.ReadFile("web/" + path)
	if err != nil {
		b, err = s.uiFS.ReadFile("web/index.html")
		if err != nil {
			http.Error(w, "ui unavailable", http.StatusInternalServerError)
			return
		}
	}
	if strings.HasSuffix(path, ".css") {
		w.Header().Set("Content-Type", "text/css")
	}
	if strings.HasSuffix(path, ".js") {
		w.Header().Set("Content-Type", "application/javascript")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if !s.auth.Login(req.Username, req.Password) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	s.auth.SetSession(w, req.Username)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.store.Alerts())
	case http.MethodPost:
		var req app.Alert
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if req.Source == "" {
			req.Source = "grafana"
		}
		alert := s.store.SaveAlert(req)
		triageReport := s.agent.Review(alert)
		s.store.UpdateAlertTriage(alert.ID, triageReport)
		alert.Triage = &triageReport

		if alert.Severity == app.SeverityCritical {
			incident := s.store.CreateIncident(app.Incident{
				AlertID:       alert.ID,
				Title:         "Auto-created incident for critical alert: " + alert.Title,
				Severity:      alert.Severity,
				Status:        "investigating",
				StatusPageURL: "/status/" + alert.ID,
			})
			writeJSON(w, http.StatusCreated, map[string]any{"alert": alert, "incident": incident})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"alert": alert})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleIncidents(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, s.store.Incidents())
		return
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handlePostMortems(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.store.PostMortems())
	case http.MethodPost:
		var pm app.PostMortem
		if err := json.NewDecoder(r.Body).Decode(&pm); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusCreated, s.store.AddPostMortem(pm))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handlePlaybooks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.store.Playbooks())
	case http.MethodPost:
		var pb app.Playbook
		if err := json.NewDecoder(r.Body).Decode(&pb); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusCreated, s.store.AddPlaybook(pb))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleOnCall(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.store.OnCall())
	case http.MethodPost:
		var shift app.OnCallShift
		if err := json.NewDecoder(r.Body).Decode(&shift); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusCreated, s.store.AddShift(shift))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
