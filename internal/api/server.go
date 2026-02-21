package api

import (
	"embed"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

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
	mux.HandleFunc("/api/logout", s.handleLogout)
	mux.HandleFunc("/api/statuspage", s.handlePublicStatusPage)

	protected := http.NewServeMux()
	protected.Handle("/api/alerts", s.auth.RequirePermission("read:dashboard", http.HandlerFunc(s.handleAlerts)))
	protected.Handle("/api/incidents", s.auth.RequirePermission("read:dashboard", http.HandlerFunc(s.handleIncidents)))
	protected.Handle("/api/postmortems", s.auth.RequirePermission("read:dashboard", http.HandlerFunc(s.handlePostMortems)))
	protected.Handle("/api/playbooks", s.auth.RequirePermission("read:dashboard", http.HandlerFunc(s.handlePlaybooks)))
	protected.Handle("/api/oncall", s.auth.RequirePermission("read:dashboard", http.HandlerFunc(s.handleOnCall)))
	protected.Handle("/api/tools", s.auth.RequirePermission("read:dashboard", http.HandlerFunc(s.handleTools)))
	protected.Handle("/api/tools/", s.auth.RequirePermission("read:dashboard", http.HandlerFunc(s.handleToolByID)))
	protected.Handle("/api/me", http.HandlerFunc(s.handleMe))

	protected.Handle("/api/admin/users", s.auth.RequirePermission("admin:users", http.HandlerFunc(s.handleAdminUsers)))
	protected.Handle("/api/admin/roles", s.auth.RequirePermission("admin:roles", http.HandlerFunc(s.handleAdminRoles)))
	protected.Handle(
		"/api/admin/invites",
		s.auth.RequirePermission("admin:invites", http.HandlerFunc(s.handleAdminInvites)),
	)

	mux.Handle("/api/", s.auth.Middleware(protected))
	mux.HandleFunc("/", s.handleUI)

	return mux
}

func (s *Server) handlePublicStatusPage(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	incidents, err := s.store.Incidents()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	services, err := s.store.Services()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	periodHours := 24
	if raw := request.URL.Query().Get("periodHours"); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr == nil && parsed > 0 && parsed <= 24*30 {
			periodHours = parsed
		}
	}
	periodStart := now.Add(-time.Duration(periodHours) * time.Hour)

	status := app.PublicStatusPage{
		OverallStatus: "operational",
		UpdatedAt:     now,
		PeriodStart:   periodStart,
		PeriodEnd:     now,
		Services:      buildServiceAvailability(services, incidents, periodStart, now),
		Incidents:     make([]app.StatusPageIncident, 0, len(incidents)),
	}

	for _, incident := range incidents {
		if incident.Status != "investigating" && incident.Status != "identified" {
			continue
		}
		status.Incidents = append(status.Incidents, app.StatusPageIncident{
			ID:             incident.ID,
			Service:        incident.Service,
			Title:          incident.Title,
			Severity:       incident.Severity,
			Status:         incident.Status,
			DeclaredAt:     incident.CreatedAt,
			StatusPageURL:  incident.StatusPageURL,
			CurrentMessage: "Incident declared. Command role assigned, communications started, mitigation in progress.",
			ResponsePlaybook: []string{
				"Assign incident commander and define communication cadence",
				"Assess customer impact against SLOs and error budget policy",
				"Stabilize service and execute mitigation plan",
				"Capture timeline and prepare blameless postmortem",
			},
		})

		if incident.Severity == app.SeverityCritical {
			status.OverallStatus = "major_outage"
		} else if status.OverallStatus == "operational" {
			status.OverallStatus = "degraded_performance"
		}
	}

	writeJSON(writer, http.StatusOK, status)
}

func (s *Server) handleUI(writer http.ResponseWriter, request *http.Request) {
	path := strings.TrimPrefix(request.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if path == "status" || strings.HasPrefix(path, "status/") {
		path = "status.html"
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

	user, err := s.store.AuthenticateUser(loginRequest.Username, loginRequest.Password)
	if err != nil {
		http.Error(writer, "invalid credentials", http.StatusUnauthorized)
		return
	}

	s.auth.SetSession(writer, user.Username, user.Roles)
	writeJSON(writer, http.StatusOK, map[string]any{
		"status":     "ok",
		"authMode":   "local",
		"ssoEnabled": false,
		"user":       user,
	})
}

func (s *Server) handleLogout(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.auth.ClearSession(writer)
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMe(writer http.ResponseWriter, request *http.Request) {
	session, ok := auth.UserFromContext(request.Context())
	if !ok {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	user, err := s.store.GetUser(session.Username)
	if err != nil {
		http.Error(writer, "user not found", http.StatusUnauthorized)
		return
	}
	user.PasswordHash = ""
	writeJSON(writer, http.StatusOK, user)
}

func (s *Server) handleAdminUsers(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		users, err := s.store.ListUsers()
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, http.StatusOK, users)
	case http.MethodPost:
		var payload struct {
			Username    string   `json:"username"`
			DisplayName string   `json:"displayName"`
			Password    string   `json:"password"`
			Roles       []string `json:"roles"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(writer, "invalid json", http.StatusBadRequest)
			return
		}
		if payload.DisplayName == "" {
			payload.DisplayName = payload.Username
		}
		created, err := s.store.CreateUser(payload.Username, payload.DisplayName, payload.Password, payload.Roles)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(writer, http.StatusCreated, created)
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAdminRoles(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		roles, err := s.store.ListRoles()
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, http.StatusOK, roles)
	case http.MethodPost:
		var payload app.Role
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(writer, "invalid json", http.StatusBadRequest)
			return
		}
		created, err := s.store.CreateRole(payload)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(writer, http.StatusCreated, created)
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAdminInvites(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		invites, err := s.store.ListInvites()
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, http.StatusOK, invites)
	case http.MethodPost:
		var payload struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(writer, "invalid json", http.StatusBadRequest)
			return
		}
		invite, err := s.store.CreateInvite(payload.Email, payload.Role)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(writer, http.StatusCreated, invite)
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
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
	if alertRequest.Status == "" {
		alertRequest.Status = "received"
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
	alert.Status = "triaged"
	if triageReport.Decision != "start_incident" {
		writeJSON(writer, http.StatusCreated, map[string]any{"alert": alert})
		return
	}

	service := "unknown"
	if alert.Labels != nil && alert.Labels["service"] != "" {
		service = alert.Labels["service"]
	}

	if _, err = s.store.EnsureService(service); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	incident, err := s.store.CreateIncident(app.Incident{
		AlertID:       alert.ID,
		Service:       service,
		Title:         "Auto-created incident for triaged alert: " + alert.Title,
		Severity:      alert.Severity,
		Status:        "investigating",
		StatusPageURL: "/status/" + alert.ID,
	})
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = s.store.UpdateAlertStatus(alert.ID, "incident_open"); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	alert.Status = "incident_open"
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

func (s *Server) handleTools(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		items, err := s.store.Tools()
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, http.StatusOK, items)
	case http.MethodPost:
		var payload app.MCPTool
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(writer, "invalid json", http.StatusBadRequest)
			return
		}
		created, err := s.store.CreateTool(payload)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(writer, http.StatusCreated, created)
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleToolByID(writer http.ResponseWriter, request *http.Request) {
	toolID := strings.TrimPrefix(request.URL.Path, "/api/tools/")
	if toolID == "" {
		http.Error(writer, "tool id required", http.StatusBadRequest)
		return
	}
	switch request.Method {
	case http.MethodGet:
		item, err := s.store.Tool(toolID)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(writer, http.StatusOK, item)
	case http.MethodPut:
		var payload app.MCPTool
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(writer, "invalid json", http.StatusBadRequest)
			return
		}
		updated, err := s.store.UpdateTool(toolID, payload)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(writer, http.StatusOK, updated)
	case http.MethodDelete:
		if err := s.store.DeleteTool(toolID); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(writer, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func buildServiceAvailability(services []app.Service, incidents []app.Incident, periodStart, periodEnd time.Time) []app.ServiceAvailability {
	serviceDowntime := map[string]time.Duration{}
	for _, service := range services {
		name := service.Name
		if name == "" {
			continue
		}
		serviceDowntime[name] = 0
	}
	periodDuration := periodEnd.Sub(periodStart)
	if periodDuration <= 0 {
		return []app.ServiceAvailability{}
	}

	for _, incident := range incidents {
		service := incident.Service
		if service == "" {
			service = "unknown"
		}
		if _, ok := serviceDowntime[service]; !ok {
			serviceDowntime[service] = 0
		}

		incidentEnd := periodEnd
		if incident.ResolvedAt != nil {
			incidentEnd = *incident.ResolvedAt
		}
		if incidentEnd.Before(periodStart) || incident.CreatedAt.After(periodEnd) {
			continue
		}
		if incident.CreatedAt.After(incidentEnd) {
			continue
		}

		start := incident.CreatedAt
		if start.Before(periodStart) {
			start = periodStart
		}
		end := incidentEnd
		if end.After(periodEnd) {
			end = periodEnd
		}
		if end.After(start) {
			serviceDowntime[service] += end.Sub(start)
		}
	}

	serviceNames := make([]string, 0, len(serviceDowntime))
	for service := range serviceDowntime {
		serviceNames = append(serviceNames, service)
	}
	sort.Strings(serviceNames)

	availabilities := make([]app.ServiceAvailability, 0, len(serviceDowntime))
	for _, service := range serviceNames {
		downtime := serviceDowntime[service]
		if downtime < 0 {
			downtime = 0
		}
		availability := 100 - (float64(downtime)/float64(periodDuration))*100
		if availability < 0 {
			availability = 0
		}
		availabilities = append(availabilities, app.ServiceAvailability{
			Service:             service,
			AvailabilityPercent: availability,
			DowntimeMinutes:     int(downtime / time.Minute),
			PeriodStart:         periodStart,
			PeriodEnd:           periodEnd,
		})
	}

	return availabilities
}

func writeJSON(writer http.ResponseWriter, status int, data any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(data); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}
