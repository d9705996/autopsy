package store

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/example/autopsy/internal/app"
)

const memoryServiceUnknown = "unknown"

var (
	errNotImplemented = errors.New("not implemented")
	errToolNotFound   = errors.New("tool not found")
)

type MemoryStore struct {
	mu        sync.RWMutex
	counter   uint64
	alerts    []app.Alert
	incidents []app.Incident
	services  []app.Service
	tools     []app.MCPTool
}

func NewMemoryStore() *MemoryStore  { return &MemoryStore{} }
func (s *MemoryStore) Close() error { return nil }

func (s *MemoryStore) nextID(prefix string) string {
	n := atomic.AddUint64(&s.counter, 1)
	return fmt.Sprintf("%s-%06d", prefix, n)
}

func (s *MemoryStore) SaveAlert(a app.Alert) (app.Alert, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a.ID = s.nextID("alt")
	a.CreatedAt = time.Now().UTC()
	if a.Status == "" {
		a.Status = "received"
	}
	s.alerts = append(s.alerts, a)
	return a, nil
}

func (s *MemoryStore) UpdateAlertTriage(alertID string, triage app.TriageReport) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.alerts {
		if s.alerts[i].ID == alertID {
			s.alerts[i].Triage = &triage
			s.alerts[i].Status = "triaged"
			return nil
		}
	}
	return nil
}

func (s *MemoryStore) UpdateAlertStatus(alertID, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.alerts {
		if s.alerts[i].ID == alertID {
			s.alerts[i].Status = status
			return nil
		}
	}
	return nil
}

func (s *MemoryStore) Alerts() ([]app.Alert, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]app.Alert, len(s.alerts))
	copy(out, s.alerts)
	return out, nil
}

func (s *MemoryStore) CreateIncident(incident app.Incident) (app.Incident, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	incident.ID = s.nextID("inc")
	if incident.CreatedAt.IsZero() {
		incident.CreatedAt = time.Now().UTC()
	}
	if incident.Service == "" {
		incident.Service = memoryServiceUnknown
	}
	hasService := false
	for _, svc := range s.services {
		if svc.Name == incident.Service {
			hasService = true
			break
		}
	}
	if !hasService {
		s.services = append(s.services, app.Service{ID: s.nextID("svc"), Name: incident.Service, CreatedAt: time.Now().UTC()})
	}
	if incident.Status == "resolved" && incident.ResolvedAt == nil {
		resolvedAt := time.Now().UTC()
		incident.ResolvedAt = &resolvedAt
	}
	s.incidents = append(s.incidents, incident)
	return incident, nil
}

func (s *MemoryStore) Incidents() ([]app.Incident, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]app.Incident, len(s.incidents))
	copy(out, s.incidents)
	return out, nil
}

func (s *MemoryStore) EnsureService(name string) (app.Service, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if name == "" {
		name = memoryServiceUnknown
	}
	for _, svc := range s.services {
		if svc.Name == name {
			return svc, nil
		}
	}
	svc := app.Service{
		ID:        s.nextID("svc"),
		Name:      name,
		CreatedAt: time.Now().UTC(),
	}
	s.services = append(s.services, svc)
	return svc, nil
}

func (s *MemoryStore) Services() ([]app.Service, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]app.Service, len(s.services))
	copy(out, s.services)
	return out, nil
}

func (s *MemoryStore) AddPostMortem(pm app.PostMortem) (app.PostMortem, error) { return pm, nil }
func (s *MemoryStore) PostMortems() ([]app.PostMortem, error)                  { return []app.PostMortem{}, nil }
func (s *MemoryStore) AddPlaybook(pb app.Playbook) (app.Playbook, error)       { return pb, nil }
func (s *MemoryStore) Playbooks() ([]app.Playbook, error)                      { return []app.Playbook{}, nil }
func (s *MemoryStore) AddShift(shift app.OnCallShift) (app.OnCallShift, error) { return shift, nil }
func (s *MemoryStore) OnCall() ([]app.OnCallShift, error)                      { return []app.OnCallShift{}, nil }

func (s *MemoryStore) CreateTool(tool app.MCPTool) (app.MCPTool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	tool.ID = s.nextID("tool")
	tool.CreatedAt = now
	tool.UpdatedAt = now
	s.tools = append(s.tools, tool)
	return tool, nil
}

func (s *MemoryStore) Tools() ([]app.MCPTool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]app.MCPTool, len(s.tools))
	copy(out, s.tools)
	return out, nil
}

func (s *MemoryStore) Tool(toolID string) (app.MCPTool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, tool := range s.tools {
		if tool.ID == toolID {
			return tool, nil
		}
	}
	return app.MCPTool{}, errToolNotFound
}

func (s *MemoryStore) UpdateTool(toolID string, tool app.MCPTool) (app.MCPTool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tools {
		if s.tools[i].ID == toolID {
			tool.ID = toolID
			tool.CreatedAt = s.tools[i].CreatedAt
			tool.UpdatedAt = time.Now().UTC()
			s.tools[i] = tool
			return tool, nil
		}
	}
	return app.MCPTool{}, errToolNotFound
}

func (s *MemoryStore) DeleteTool(toolID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tools {
		if s.tools[i].ID == toolID {
			s.tools = append(s.tools[:i], s.tools[i+1:]...)
			return nil
		}
	}
	return errToolNotFound
}

func (s *MemoryStore) EnsureRole(_ app.Role) error       { return nil }
func (s *MemoryStore) EnsureAdminUser(_, _ string) error { return nil }
func (s *MemoryStore) AuthenticateUser(_, _ string) (app.User, error) {
	return app.User{}, errNotImplemented
}
func (s *MemoryStore) GetUser(_ string) (app.User, error)         { return app.User{}, errNotImplemented }
func (s *MemoryStore) UserPermissions(_ string) ([]string, error) { return []string{}, nil }
func (s *MemoryStore) ListUsers() ([]app.User, error)             { return []app.User{}, nil }

func (s *MemoryStore) CreateUser(_, _, _ string, _ []string) (app.User, error) {
	return app.User{}, errNotImplemented
}

func (s *MemoryStore) ListRoles() ([]app.Role, error)               { return []app.Role{}, nil }
func (s *MemoryStore) CreateRole(role app.Role) (app.Role, error)   { return role, nil }
func (s *MemoryStore) CreateInvite(_, _ string) (app.Invite, error) { return app.Invite{}, nil }
func (s *MemoryStore) ListInvites() ([]app.Invite, error)           { return []app.Invite{}, nil }
