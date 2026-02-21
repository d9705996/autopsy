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
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Payload     map[string]any    `json:"payload"`
	CreatedAt   time.Time         `json:"createdAt"`
	Triage      *TriageReport     `json:"triage,omitempty"`
}

type TriageTimelineStep struct {
	Phase     string    `json:"phase"`
	Detail    string    `json:"detail"`
	Timestamp time.Time `json:"timestamp"`
}

type TriageReport struct {
	Summary          string               `json:"summary"`
	LikelyRootCause  string               `json:"likelyRootCause"`
	SuggestedActions []string             `json:"suggestedActions"`
	Decision         string               `json:"decision"`
	IssueTitle       string               `json:"issueTitle,omitempty"`
	AutoFixPlan      []string             `json:"autoFixPlan,omitempty"`
	Timeline         []TriageTimelineStep `json:"timeline"`
	Confidence       string               `json:"confidence"`
	ReviewedAt       time.Time            `json:"reviewedAt"`
}

type Service struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Incident struct {
	ID            string     `json:"id"`
	AlertID       string     `json:"alertId"`
	Service       string     `json:"service"`
	Title         string     `json:"title"`
	Severity      Severity   `json:"severity"`
	Status        string     `json:"status"`
	StatusPageURL string     `json:"statusPageUrl"`
	CreatedAt     time.Time  `json:"createdAt"`
	ResolvedAt    *time.Time `json:"resolvedAt,omitempty"`
}

type StatusPageIncident struct {
	ID               string    `json:"id"`
	Service          string    `json:"service"`
	Title            string    `json:"title"`
	Severity         Severity  `json:"severity"`
	Status           string    `json:"status"`
	DeclaredAt       time.Time `json:"declaredAt"`
	StatusPageURL    string    `json:"statusPageUrl"`
	CurrentMessage   string    `json:"currentMessage"`
	ResponsePlaybook []string  `json:"responsePlaybook"`
}

type ServiceAvailability struct {
	Service             string    `json:"service"`
	AvailabilityPercent float64   `json:"availabilityPercent"`
	DowntimeMinutes     int       `json:"downtimeMinutes"`
	PeriodStart         time.Time `json:"periodStart"`
	PeriodEnd           time.Time `json:"periodEnd"`
}

type PublicStatusPage struct {
	OverallStatus string                `json:"overallStatus"`
	UpdatedAt     time.Time             `json:"updatedAt"`
	PeriodStart   time.Time             `json:"periodStart"`
	PeriodEnd     time.Time             `json:"periodEnd"`
	Services      []ServiceAvailability `json:"services"`
	Incidents     []StatusPageIncident  `json:"incidents"`
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

type MCPTool struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Server      string            `json:"server"`
	Tool        string            `json:"tool"`
	Config      map[string]string `json:"config"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	DisplayName  string    `json:"displayName"`
	PasswordHash string    `json:"-"`
	Roles        []string  `json:"roles"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Role struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Invite struct {
	ID         int64      `json:"id"`
	Email      string     `json:"email"`
	Role       string     `json:"role"`
	Token      string     `json:"token"`
	Status     string     `json:"status"`
	ExpiresAt  time.Time  `json:"expiresAt"`
	AcceptedAt *time.Time `json:"acceptedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}
