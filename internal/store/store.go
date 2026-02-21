package store

import "github.com/example/autopsy/internal/app"

type Repository interface {
	SaveAlert(a app.Alert) (app.Alert, error)
	UpdateAlertTriage(alertID string, triage app.TriageReport) error
	Alerts() ([]app.Alert, error)
	CreateIncident(incident app.Incident) (app.Incident, error)
	Incidents() ([]app.Incident, error)
	AddPostMortem(pm app.PostMortem) (app.PostMortem, error)
	PostMortems() ([]app.PostMortem, error)
	AddPlaybook(pb app.Playbook) (app.Playbook, error)
	Playbooks() ([]app.Playbook, error)
	AddShift(shift app.OnCallShift) (app.OnCallShift, error)
	OnCall() ([]app.OnCallShift, error)
	Close() error
}
