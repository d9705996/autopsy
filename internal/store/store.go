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

	EnsureRole(role app.Role) error
	EnsureAdminUser(username, password string) error
	AuthenticateUser(username, password string) (app.User, error)
	GetUser(username string) (app.User, error)
	ListUsers() ([]app.User, error)
	CreateUser(username, displayName, password string, roles []string) (app.User, error)
	ListRoles() ([]app.Role, error)
	CreateRole(role app.Role) (app.Role, error)
	CreateInvite(email, role string) (app.Invite, error)
	ListInvites() ([]app.Invite, error)

	Close() error
}
