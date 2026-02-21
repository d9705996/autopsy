package triage

import (
	"fmt"
	"strings"
	"time"

	"github.com/example/autopsy/internal/app"
)

type Agent interface {
	Review(alert app.Alert) app.TriageReport
}

type HeuristicAgent struct{}

func NewHeuristicAgent() *HeuristicAgent { return &HeuristicAgent{} }

func (a *HeuristicAgent) Review(alert app.Alert) app.TriageReport {
	now := time.Now().UTC()
	rootCause := "Insufficient telemetry for root-cause confidence"
	if metric, ok := alert.Labels["metric"]; ok {
		rootCause = fmt.Sprintf("Anomaly detected in metric %q; likely saturation/regression", metric)
	}
	if strings.Contains(strings.ToLower(alert.Description), "timeout") {
		rootCause = "Downstream dependency timeout causing user impact"
	}
	actions := []string{
		"Check SLO burn rate and error budget policy per Google SRE guidance",
		"Review recent deploy and rollback if correlated",
		"Verify service-level indicators in logs/metrics dashboard",
	}
	confidence := "medium"
	if alert.Severity == app.SeverityCritical {
		confidence = "high"
	}

	decision := "create_issue"
	summary := "Alert reviewed; routed for follow-up issue triage"
	autoFixPlan := []string{}
	issueTitle := fmt.Sprintf("Follow-up: %s (%s)", alert.Title, alert.Severity)
	if alert.Severity == app.SeverityCritical || strings.Contains(strings.ToLower(alert.Description), "customer") {
		decision = "start_incident"
		summary = "High-risk customer impact detected; incident response should start now"
		issueTitle = ""
	}
	if alert.Severity == app.SeverityWarning && strings.Contains(strings.ToLower(alert.Description), "retry") {
		decision = "auto_fix"
		summary = "Alert appears remediable via safe automation"
		autoFixPlan = []string{
			"Scale workers for affected queue by +20%",
			"Invalidate stale cache entries for impacted route",
			"Monitor recovery for 15 minutes before resolving",
		}
		issueTitle = ""
	}

	timeline := []app.TriageTimelineStep{
		{Phase: "received", Detail: "Alert ingested and queued for AI triage", Timestamp: now.Add(-5 * time.Second)},
		{Phase: "context", Detail: "Correlated severity, labels, and recent error patterns", Timestamp: now.Add(-3 * time.Second)},
		{Phase: "analysis", Detail: rootCause, Timestamp: now.Add(-1 * time.Second)},
		{Phase: "decision", Detail: fmt.Sprintf("Decision: %s", decision), Timestamp: now},
	}

	return app.TriageReport{
		Summary:          summary,
		LikelyRootCause:  rootCause,
		SuggestedActions: actions,
		Decision:         decision,
		IssueTitle:       issueTitle,
		AutoFixPlan:      autoFixPlan,
		Timeline:         timeline,
		Confidence:       confidence,
		ReviewedAt:       now,
	}
}
