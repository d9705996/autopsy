package app

import "time"

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type Alert struct {
	ID          string            `json:"id"`
	Source      string            `json:"source"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Severity    Severity          `json:"severity"`
	Labels      map[string]string `json:"labels"`
	Payload     map[string]any    `json:"payload"`
	CreatedAt   time.Time         `json:"createdAt"`
	Triage      *TriageReport     `json:"triage,omitempty"`
}

type TriageReport struct {
	Summary          string    `json:"summary"`
	LikelyRootCause  string    `json:"likelyRootCause"`
	SuggestedActions []string  `json:"suggestedActions"`
	Confidence       string    `json:"confidence"`
	ReviewedAt       time.Time `json:"reviewedAt"`
}

type Incident struct {
	ID            string    `json:"id"`
	AlertID       string    `json:"alertId"`
	Title         string    `json:"title"`
	Severity      Severity  `json:"severity"`
	Status        string    `json:"status"`
	StatusPageURL string    `json:"statusPageUrl"`
	CreatedAt     time.Time `json:"createdAt"`
}

type PostMortem struct {
	ID         string    `json:"id"`
	IncidentID string    `json:"incidentId"`
	Summary    string    `json:"summary"`
	Timeline   []string  `json:"timeline"`
	Learnings  []string  `json:"learnings"`
	Actions    []string  `json:"actions"`
	CreatedAt  time.Time `json:"createdAt"`
}

type Playbook struct {
	ID          string    `json:"id"`
	Service     string    `json:"service"`
	Title       string    `json:"title"`
	Steps       []string  `json:"steps"`
	LastUpdated time.Time `json:"lastUpdated"`
}

type OnCallShift struct {
	ID         string    `json:"id"`
	Engineer   string    `json:"engineer"`
	PrimaryFor string    `json:"primaryFor"`
	Start      time.Time `json:"start"`
	End        time.Time `json:"end"`
	Escalation []string  `json:"escalation"`
}

type User struct {
	Username string
	Password string
	Role     string
}
