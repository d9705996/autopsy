package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/example/autopsy/internal/app"
)

const postgresDialect = "postgres"

type SQLStore struct {
	db       *sql.DB
	dialect  string
	nowClock func() time.Time
}

func NewSQLStore(driver, dsn string) (*SQLStore, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	s := &SQLStore{db: db, nowClock: func() time.Time { return time.Now().UTC() }}
	if driver == "sqlite" {
		s.dialect = "sqlite"
	} else {
		s.dialect = postgresDialect
	}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *SQLStore) Close() error { return s.db.Close() }

func (s *SQLStore) placeholder(n int) string {
	if s.dialect == postgresDialect {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

func (s *SQLStore) migrate() error {
	var stmts []string
	if s.dialect == postgresDialect {
		stmts = []string{
			`CREATE TABLE IF NOT EXISTS alerts (
				id BIGSERIAL PRIMARY KEY,
				source TEXT NOT NULL,
				title TEXT NOT NULL,
				description TEXT NOT NULL,
				severity TEXT NOT NULL,
				labels TEXT,
				payload TEXT,
				triage TEXT,
				created_at TIMESTAMP NOT NULL
			);`,
			`CREATE TABLE IF NOT EXISTS incidents (
				id BIGSERIAL PRIMARY KEY,
				alert_id TEXT NOT NULL,
				title TEXT NOT NULL,
				severity TEXT NOT NULL,
				status TEXT NOT NULL,
				status_page_url TEXT NOT NULL,
				created_at TIMESTAMP NOT NULL
			);`,
			`CREATE TABLE IF NOT EXISTS postmortems (
				id BIGSERIAL PRIMARY KEY,
				incident_id TEXT NOT NULL,
				summary TEXT NOT NULL,
				timeline TEXT,
				learnings TEXT,
				actions TEXT,
				created_at TIMESTAMP NOT NULL
			);`,
			`CREATE TABLE IF NOT EXISTS playbooks (
				id BIGSERIAL PRIMARY KEY,
				service TEXT NOT NULL,
				title TEXT NOT NULL,
				steps TEXT,
				last_updated TIMESTAMP NOT NULL
			);`,
			`CREATE TABLE IF NOT EXISTS oncall_shifts (
				id BIGSERIAL PRIMARY KEY,
				engineer TEXT NOT NULL,
				primary_for TEXT NOT NULL,
				start_at TIMESTAMP NOT NULL,
				end_at TIMESTAMP NOT NULL,
				escalation TEXT
			);`,
		}
	} else {
		stmts = []string{
			`CREATE TABLE IF NOT EXISTS alerts (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				source TEXT NOT NULL,
				title TEXT NOT NULL,
				description TEXT NOT NULL,
				severity TEXT NOT NULL,
				labels TEXT,
				payload TEXT,
				triage TEXT,
				created_at TIMESTAMP NOT NULL
			);`,
			`CREATE TABLE IF NOT EXISTS incidents (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				alert_id TEXT NOT NULL,
				title TEXT NOT NULL,
				severity TEXT NOT NULL,
				status TEXT NOT NULL,
				status_page_url TEXT NOT NULL,
				created_at TIMESTAMP NOT NULL
			);`,
			`CREATE TABLE IF NOT EXISTS postmortems (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				incident_id TEXT NOT NULL,
				summary TEXT NOT NULL,
				timeline TEXT,
				learnings TEXT,
				actions TEXT,
				created_at TIMESTAMP NOT NULL
			);`,
			`CREATE TABLE IF NOT EXISTS playbooks (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				service TEXT NOT NULL,
				title TEXT NOT NULL,
				steps TEXT,
				last_updated TIMESTAMP NOT NULL
			);`,
			`CREATE TABLE IF NOT EXISTS oncall_shifts (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				engineer TEXT NOT NULL,
				primary_for TEXT NOT NULL,
				start_at TIMESTAMP NOT NULL,
				end_at TIMESTAMP NOT NULL,
				escalation TEXT
			);`,
		}
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLStore) insertWithID(baseInsert string, args ...any) (int64, error) {
	if s.dialect == postgresDialect {
		q := baseInsert + " RETURNING id"
		var id int64
		if err := s.db.QueryRow(q, args...).Scan(&id); err != nil {
			return 0, err
		}
		return id, nil
	}
	res, err := s.db.Exec(baseInsert, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func marshalJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal JSON: %w", err)
	}

	return string(b), nil
}

func (s *SQLStore) SaveAlert(a app.Alert) (app.Alert, error) {
	a.CreatedAt = s.nowClock()
	labelsJSON, err := marshalJSON(a.Labels)
	if err != nil {
		return app.Alert{}, err
	}
	payloadJSON, err := marshalJSON(a.Payload)
	if err != nil {
		return app.Alert{}, err
	}

	q := `INSERT INTO alerts (source,title,description,severity,labels,payload,created_at) VALUES (%s,%s,%s,%s,%s,%s,%s)`
	q = fmt.Sprintf(q, s.placeholder(1), s.placeholder(2), s.placeholder(3), s.placeholder(4), s.placeholder(5), s.placeholder(6), s.placeholder(7))
	id, err := s.insertWithID(q, a.Source, a.Title, a.Description, string(a.Severity), labelsJSON, payloadJSON, a.CreatedAt)
	if err != nil {
		return app.Alert{}, err
	}
	a.ID = fmt.Sprintf("alt-%06d", id)
	return a, nil
}

func (s *SQLStore) UpdateAlertTriage(alertID string, triage app.TriageReport) error {
	triageJSON, err := marshalJSON(triage)
	if err != nil {
		return err
	}

	q := `UPDATE alerts SET triage=%s WHERE id=%s`
	q = fmt.Sprintf(q, s.placeholder(1), s.placeholder(2))
	_, err = s.db.Exec(q, triageJSON, parseNumericID(alertID))
	return err
}

func (s *SQLStore) Alerts() ([]app.Alert, error) {
	rows, err := s.db.Query(`SELECT id,source,title,description,severity,labels,payload,triage,created_at FROM alerts ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []app.Alert{}
	for rows.Next() {
		var id int64
		var severity, labels, payload string
		var triage sql.NullString
		var a app.Alert
		if err := rows.Scan(&id, &a.Source, &a.Title, &a.Description, &severity, &labels, &payload, &triage, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.ID = fmt.Sprintf("alt-%06d", id)
		a.Severity = app.Severity(severity)
		_ = json.Unmarshal([]byte(labels), &a.Labels)
		_ = json.Unmarshal([]byte(payload), &a.Payload)
		if triage.Valid && triage.String != "" {
			var tr app.TriageReport
			_ = json.Unmarshal([]byte(triage.String), &tr)
			a.Triage = &tr
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *SQLStore) CreateIncident(in app.Incident) (app.Incident, error) {
	in.CreatedAt = s.nowClock()
	q := `INSERT INTO incidents (alert_id,title,severity,status,status_page_url,created_at) VALUES (%s,%s,%s,%s,%s,%s)`
	q = fmt.Sprintf(q, s.placeholder(1), s.placeholder(2), s.placeholder(3), s.placeholder(4), s.placeholder(5), s.placeholder(6))
	id, err := s.insertWithID(q, in.AlertID, in.Title, string(in.Severity), in.Status, in.StatusPageURL, in.CreatedAt)
	if err != nil {
		return app.Incident{}, err
	}
	in.ID = fmt.Sprintf("inc-%06d", id)
	return in, nil
}

func (s *SQLStore) Incidents() ([]app.Incident, error) {
	rows, err := s.db.Query(`SELECT id,alert_id,title,severity,status,status_page_url,created_at FROM incidents ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []app.Incident
	for rows.Next() {
		var id int64
		var sev string
		var in app.Incident
		if err := rows.Scan(&id, &in.AlertID, &in.Title, &sev, &in.Status, &in.StatusPageURL, &in.CreatedAt); err != nil {
			return nil, err
		}
		in.ID = fmt.Sprintf("inc-%06d", id)
		in.Severity = app.Severity(sev)
		out = append(out, in)
	}
	return out, rows.Err()
}

func (s *SQLStore) AddPostMortem(pm app.PostMortem) (app.PostMortem, error) {
	pm.CreatedAt = s.nowClock()
	timelineJSON, err := marshalJSON(pm.Timeline)
	if err != nil {
		return app.PostMortem{}, err
	}
	learningsJSON, err := marshalJSON(pm.Learnings)
	if err != nil {
		return app.PostMortem{}, err
	}
	actionsJSON, err := marshalJSON(pm.Actions)
	if err != nil {
		return app.PostMortem{}, err
	}

	q := `INSERT INTO postmortems (incident_id,summary,timeline,learnings,actions,created_at) VALUES (%s,%s,%s,%s,%s,%s)`
	q = fmt.Sprintf(q, s.placeholder(1), s.placeholder(2), s.placeholder(3), s.placeholder(4), s.placeholder(5), s.placeholder(6))
	id, err := s.insertWithID(q, pm.IncidentID, pm.Summary, timelineJSON, learningsJSON, actionsJSON, pm.CreatedAt)
	if err != nil {
		return app.PostMortem{}, err
	}
	pm.ID = fmt.Sprintf("pm-%06d", id)
	return pm, nil
}

func (s *SQLStore) PostMortems() ([]app.PostMortem, error) {
	rows, err := s.db.Query(`SELECT id,incident_id,summary,timeline,learnings,actions,created_at FROM postmortems ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []app.PostMortem
	for rows.Next() {
		var id int64
		var t, l, a string
		var pm app.PostMortem
		if err := rows.Scan(&id, &pm.IncidentID, &pm.Summary, &t, &l, &a, &pm.CreatedAt); err != nil {
			return nil, err
		}
		pm.ID = fmt.Sprintf("pm-%06d", id)
		_ = json.Unmarshal([]byte(t), &pm.Timeline)
		_ = json.Unmarshal([]byte(l), &pm.Learnings)
		_ = json.Unmarshal([]byte(a), &pm.Actions)
		out = append(out, pm)
	}
	return out, rows.Err()
}

func (s *SQLStore) AddPlaybook(pb app.Playbook) (app.Playbook, error) {
	pb.LastUpdated = s.nowClock()
	stepsJSON, err := marshalJSON(pb.Steps)
	if err != nil {
		return app.Playbook{}, err
	}

	q := `INSERT INTO playbooks (service,title,steps,last_updated) VALUES (%s,%s,%s,%s)`
	q = fmt.Sprintf(q, s.placeholder(1), s.placeholder(2), s.placeholder(3), s.placeholder(4))
	id, err := s.insertWithID(q, pb.Service, pb.Title, stepsJSON, pb.LastUpdated)
	if err != nil {
		return app.Playbook{}, err
	}
	pb.ID = fmt.Sprintf("pb-%06d", id)
	return pb, nil
}

func (s *SQLStore) Playbooks() ([]app.Playbook, error) {
	rows, err := s.db.Query(`SELECT id,service,title,steps,last_updated FROM playbooks ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []app.Playbook
	for rows.Next() {
		var id int64
		var steps string
		var pb app.Playbook
		if err := rows.Scan(&id, &pb.Service, &pb.Title, &steps, &pb.LastUpdated); err != nil {
			return nil, err
		}
		pb.ID = fmt.Sprintf("pb-%06d", id)
		_ = json.Unmarshal([]byte(steps), &pb.Steps)
		out = append(out, pb)
	}
	return out, rows.Err()
}

func (s *SQLStore) AddShift(shift app.OnCallShift) (app.OnCallShift, error) {
	escalationJSON, err := marshalJSON(shift.Escalation)
	if err != nil {
		return app.OnCallShift{}, err
	}

	q := `INSERT INTO oncall_shifts (engineer,primary_for,start_at,end_at,escalation) VALUES (%s,%s,%s,%s,%s)`
	q = fmt.Sprintf(q, s.placeholder(1), s.placeholder(2), s.placeholder(3), s.placeholder(4), s.placeholder(5))
	id, err := s.insertWithID(q, shift.Engineer, shift.PrimaryFor, shift.Start, shift.End, escalationJSON)
	if err != nil {
		return app.OnCallShift{}, err
	}
	shift.ID = fmt.Sprintf("oc-%06d", id)
	return shift, nil
}

func (s *SQLStore) OnCall() ([]app.OnCallShift, error) {
	rows, err := s.db.Query(`SELECT id,engineer,primary_for,start_at,end_at,escalation FROM oncall_shifts ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []app.OnCallShift
	for rows.Next() {
		var id int64
		var esc string
		var sh app.OnCallShift
		if err := rows.Scan(&id, &sh.Engineer, &sh.PrimaryFor, &sh.Start, &sh.End, &esc); err != nil {
			return nil, err
		}
		sh.ID = fmt.Sprintf("oc-%06d", id)
		_ = json.Unmarshal([]byte(esc), &sh.Escalation)
		out = append(out, sh)
	}
	return out, rows.Err()
}

func parseNumericID(prefixed string) int64 {
	var prefix string
	var id int64
	_, _ = fmt.Sscanf(prefixed, "%3s-%d", &prefix, &id)
	return id
}
