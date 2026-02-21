package store

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/example/autopsy/internal/app"
)

type MemoryStore struct {
	mu         sync.RWMutex
	alerts     []app.Alert
	incidents  []app.Incident
	postMortem []app.PostMortem
	playbooks  []app.Playbook
	onCall     []app.OnCallShift
	counter    uint64
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (s *MemoryStore) nextID(prefix string) string {
	n := atomic.AddUint64(&s.counter, 1)
	return fmt.Sprintf("%s-%06d", prefix, n)
}

func (s *MemoryStore) SaveAlert(a app.Alert) app.Alert {
	s.mu.Lock()
	defer s.mu.Unlock()
	a.ID = s.nextID("alt")
	a.CreatedAt = time.Now().UTC()
	s.alerts = append(s.alerts, a)
	return a
}

func (s *MemoryStore) UpdateAlertTriage(alertID string, triage app.TriageReport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.alerts {
		if s.alerts[i].ID == alertID {
			s.alerts[i].Triage = &triage
			return
		}
	}
}

func (s *MemoryStore) Alerts() []app.Alert {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make([]app.Alert, len(s.alerts))
	copy(cp, s.alerts)
	return cp
}

func (s *MemoryStore) CreateIncident(incident app.Incident) app.Incident {
	s.mu.Lock()
	defer s.mu.Unlock()
	incident.ID = s.nextID("inc")
	incident.CreatedAt = time.Now().UTC()
	s.incidents = append(s.incidents, incident)
	return incident
}

func (s *MemoryStore) Incidents() []app.Incident {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make([]app.Incident, len(s.incidents))
	copy(cp, s.incidents)
	return cp
}

func (s *MemoryStore) AddPostMortem(pm app.PostMortem) app.PostMortem {
	s.mu.Lock()
	defer s.mu.Unlock()
	pm.ID = s.nextID("pm")
	pm.CreatedAt = time.Now().UTC()
	s.postMortem = append(s.postMortem, pm)
	return pm
}

func (s *MemoryStore) PostMortems() []app.PostMortem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make([]app.PostMortem, len(s.postMortem))
	copy(cp, s.postMortem)
	return cp
}

func (s *MemoryStore) AddPlaybook(pb app.Playbook) app.Playbook {
	s.mu.Lock()
	defer s.mu.Unlock()
	pb.ID = s.nextID("pb")
	pb.LastUpdated = time.Now().UTC()
	s.playbooks = append(s.playbooks, pb)
	return pb
}

func (s *MemoryStore) Playbooks() []app.Playbook {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make([]app.Playbook, len(s.playbooks))
	copy(cp, s.playbooks)
	return cp
}

func (s *MemoryStore) AddShift(shift app.OnCallShift) app.OnCallShift {
	s.mu.Lock()
	defer s.mu.Unlock()
	shift.ID = s.nextID("oc")
	s.onCall = append(s.onCall, shift)
	return shift
}

func (s *MemoryStore) OnCall() []app.OnCallShift {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make([]app.OnCallShift, len(s.onCall))
	copy(cp, s.onCall)
	return cp
}
