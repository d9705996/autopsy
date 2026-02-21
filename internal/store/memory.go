package store

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/example/autopsy/internal/app"
)

type MemoryStore struct {
	mu      sync.RWMutex
	counter uint64
	alerts  []app.Alert
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
	s.alerts = append(s.alerts, a)
	return a, nil
}
func (s *MemoryStore) UpdateAlertTriage(alertID string, triage app.TriageReport) error { return nil }
func (s *MemoryStore) Alerts() ([]app.Alert, error)                                    { return s.alerts, nil }
func (s *MemoryStore) CreateIncident(incident app.Incident) (app.Incident, error) {
	incident.ID = s.nextID("inc")
	return incident, nil
}
func (s *MemoryStore) Incidents() ([]app.Incident, error)                      { return []app.Incident{}, nil }
func (s *MemoryStore) AddPostMortem(pm app.PostMortem) (app.PostMortem, error) { return pm, nil }
func (s *MemoryStore) PostMortems() ([]app.PostMortem, error)                  { return []app.PostMortem{}, nil }
func (s *MemoryStore) AddPlaybook(pb app.Playbook) (app.Playbook, error)       { return pb, nil }
func (s *MemoryStore) Playbooks() ([]app.Playbook, error)                      { return []app.Playbook{}, nil }
func (s *MemoryStore) AddShift(shift app.OnCallShift) (app.OnCallShift, error) { return shift, nil }
func (s *MemoryStore) OnCall() ([]app.OnCallShift, error)                      { return []app.OnCallShift{}, nil }
func (s *MemoryStore) EnsureRole(role app.Role) error                          { return nil }
func (s *MemoryStore) EnsureAdminUser(username, password string) error         { return nil }
func (s *MemoryStore) AuthenticateUser(username, password string) (app.User, error) {
	return app.User{}, errors.New("not implemented")
}
func (s *MemoryStore) GetUser(username string) (app.User, error) {
	return app.User{}, errors.New("not implemented")
}
func (s *MemoryStore) ListUsers() ([]app.User, error) { return []app.User{}, nil }
func (s *MemoryStore) CreateUser(username, displayName, password string, roles []string) (app.User, error) {
	return app.User{}, errors.New("not implemented")
}
func (s *MemoryStore) ListRoles() ([]app.Role, error)                      { return []app.Role{}, nil }
func (s *MemoryStore) CreateRole(role app.Role) (app.Role, error)          { return role, nil }
func (s *MemoryStore) CreateInvite(email, role string) (app.Invite, error) { return app.Invite{}, nil }
func (s *MemoryStore) ListInvites() ([]app.Invite, error)                  { return []app.Invite{}, nil }
