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
	return app.TriageReport{
		Summary:          "AI triage completed with contextual operational hints",
		LikelyRootCause:  rootCause,
		SuggestedActions: actions,
		Confidence:       confidence,
		ReviewedAt:       time.Now().UTC(),
	}
}
