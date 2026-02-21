package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/example/autopsy/internal/app"
	"golang.org/x/crypto/bcrypt"
)

const postgresDialect = "postgres"

var (
	errRoleNameRequired = errors.New("role name is required")
	errUserDisabled     = errors.New("user disabled")
	errInvalidCreds     = errors.New("invalid credentials")
)

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
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS alerts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT NOT NULL,
			severity TEXT NOT NULL,
			status TEXT NOT NULL,
			labels TEXT,
			payload TEXT,
			triage TEXT,
			created_at TIMESTAMP NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS incidents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			alert_id TEXT NOT NULL,
			service TEXT NOT NULL DEFAULT 'unknown',
			title TEXT NOT NULL,
			severity TEXT NOT NULL,
			status TEXT NOT NULL,
			status_page_url TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			resolved_at TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS services (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT '',
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
		`CREATE TABLE IF NOT EXISTS tools (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT NOT NULL,
			server TEXT NOT NULL,
			tool TEXT NOT NULL,
			config TEXT,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			enabled BOOLEAN NOT NULL DEFAULT 1,
			created_at TIMESTAMP NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS roles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL,
			permissions TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS user_roles (
			user_id INTEGER NOT NULL,
			role_id INTEGER NOT NULL,
			PRIMARY KEY (user_id, role_id)
		);`,
		`CREATE TABLE IF NOT EXISTS invites (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL,
			role_name TEXT NOT NULL,
			token TEXT NOT NULL UNIQUE,
			status TEXT NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			accepted_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL
		);`,
	}

	if s.dialect == postgresDialect {
		// Keep compatibility with existing migration path by executing postgres specific DDL below.
		stmts = []string{
			`CREATE TABLE IF NOT EXISTS alerts (id BIGSERIAL PRIMARY KEY,source TEXT NOT NULL,title TEXT NOT NULL,description TEXT NOT NULL,severity TEXT NOT NULL,status TEXT NOT NULL,labels TEXT,payload TEXT,triage TEXT,created_at TIMESTAMP NOT NULL);`,
			`CREATE TABLE IF NOT EXISTS incidents (id BIGSERIAL PRIMARY KEY,alert_id TEXT NOT NULL,service TEXT NOT NULL DEFAULT 'unknown',title TEXT NOT NULL,severity TEXT NOT NULL,status TEXT NOT NULL,status_page_url TEXT NOT NULL,created_at TIMESTAMP NOT NULL,resolved_at TIMESTAMP);`,
			`CREATE TABLE IF NOT EXISTS services (id BIGSERIAL PRIMARY KEY,name TEXT NOT NULL UNIQUE,description TEXT NOT NULL DEFAULT '',created_at TIMESTAMP NOT NULL);`,
			`CREATE TABLE IF NOT EXISTS postmortems (id BIGSERIAL PRIMARY KEY,incident_id TEXT NOT NULL,summary TEXT NOT NULL,timeline TEXT,learnings TEXT,actions TEXT,created_at TIMESTAMP NOT NULL);`,
			`CREATE TABLE IF NOT EXISTS playbooks (id BIGSERIAL PRIMARY KEY,service TEXT NOT NULL,title TEXT NOT NULL,steps TEXT,last_updated TIMESTAMP NOT NULL);`,
			`CREATE TABLE IF NOT EXISTS oncall_shifts (id BIGSERIAL PRIMARY KEY,engineer TEXT NOT NULL,primary_for TEXT NOT NULL,start_at TIMESTAMP NOT NULL,end_at TIMESTAMP NOT NULL,escalation TEXT);`,
			`CREATE TABLE IF NOT EXISTS tools (id BIGSERIAL PRIMARY KEY,name TEXT NOT NULL,description TEXT NOT NULL,server TEXT NOT NULL,tool TEXT NOT NULL,config TEXT,created_at TIMESTAMP NOT NULL,updated_at TIMESTAMP NOT NULL);`,
			`CREATE TABLE IF NOT EXISTS users (id BIGSERIAL PRIMARY KEY,username TEXT NOT NULL UNIQUE,display_name TEXT NOT NULL,password_hash TEXT NOT NULL,enabled BOOLEAN NOT NULL DEFAULT TRUE,created_at TIMESTAMP NOT NULL);`,
			`CREATE TABLE IF NOT EXISTS roles (id BIGSERIAL PRIMARY KEY,name TEXT NOT NULL UNIQUE,description TEXT NOT NULL,permissions TEXT NOT NULL,created_at TIMESTAMP NOT NULL);`,
			`CREATE TABLE IF NOT EXISTS user_roles (user_id BIGINT NOT NULL,role_id BIGINT NOT NULL,PRIMARY KEY (user_id, role_id));`,
			`CREATE TABLE IF NOT EXISTS invites (id BIGSERIAL PRIMARY KEY,email TEXT NOT NULL,role_name TEXT NOT NULL,token TEXT NOT NULL UNIQUE,status TEXT NOT NULL,expires_at TIMESTAMP NOT NULL,accepted_at TIMESTAMP,created_at TIMESTAMP NOT NULL);`,
		}
	}

	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	if err := s.ensureAlertsStatusColumn(); err != nil {
		return err
	}
	if err := s.ensureIncidentsServiceColumn(); err != nil {
		return err
	}
	if err := s.ensureIncidentsResolvedAtColumn(); err != nil {
		return err
	}
	return nil
}

func (s *SQLStore) ensureAlertsStatusColumn() error {
	if s.dialect == postgresDialect {
		_, err := s.db.Exec(`ALTER TABLE alerts ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'received'`)
		return err
	}
	_, err := s.db.Exec(`ALTER TABLE alerts ADD COLUMN status TEXT NOT NULL DEFAULT 'received'`)
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
		return err
	}
	return nil
}

func (s *SQLStore) ensureIncidentsServiceColumn() error {
	if s.dialect == postgresDialect {
		_, err := s.db.Exec(`ALTER TABLE incidents ADD COLUMN IF NOT EXISTS service TEXT NOT NULL DEFAULT 'unknown'`)
		return err
	}
	_, err := s.db.Exec(`ALTER TABLE incidents ADD COLUMN service TEXT NOT NULL DEFAULT 'unknown'`)
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
		return err
	}
	return nil
}

func (s *SQLStore) ensureIncidentsResolvedAtColumn() error {
	if s.dialect == postgresDialect {
		_, err := s.db.Exec(`ALTER TABLE incidents ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMP`)
		return err
	}
	_, err := s.db.Exec(`ALTER TABLE incidents ADD COLUMN resolved_at TIMESTAMP`)
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
		return err
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

func (s *SQLStore) EnsureRole(role app.Role) error {
	if role.Name == "" {
		return errRoleNameRequired
	}
	if len(role.Permissions) == 0 {
		role.Permissions = []string{"read:dashboard"}
	}
	permissions, err := marshalJSON(role.Permissions)
	if err != nil {
		return err
	}
	// #nosec G201 -- placeholders are generated internally for driver compatibility.
	q := fmt.Sprintf(`INSERT INTO roles (name,description,permissions,created_at) VALUES (%s,%s,%s,%s)`, s.placeholder(1), s.placeholder(2), s.placeholder(3), s.placeholder(4))
	if s.dialect == postgresDialect {
		q += ` ON CONFLICT (name) DO NOTHING`
	} else {
		q = strings.Replace(q, "INSERT INTO", "INSERT OR IGNORE INTO", 1)
	}
	_, err = s.db.Exec(q, role.Name, role.Description, permissions, s.nowClock())
	return err
}

func (s *SQLStore) EnsureAdminUser(username, password string) error {
	if err := s.EnsureRole(app.Role{Name: "admin", Description: "System administrator", Permissions: []string{"*"}}); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	// #nosec G201 -- placeholders are generated internally for driver compatibility.
	q := fmt.Sprintf(`INSERT INTO users (username,display_name,password_hash,enabled,created_at) VALUES (%s,%s,%s,%s,%s)`, s.placeholder(1), s.placeholder(2), s.placeholder(3), s.placeholder(4), s.placeholder(5))
	if s.dialect == postgresDialect {
		q += ` ON CONFLICT (username) DO NOTHING`
	} else {
		q = strings.Replace(q, "INSERT INTO", "INSERT OR IGNORE INTO", 1)
	}
	if _, err = s.db.Exec(q, username, "Administrator", string(hash), true, s.nowClock()); err != nil {
		return err
	}
	user, err := s.GetUser(username)
	if err != nil {
		return err
	}
	return s.assignRole(user.ID, "admin")
}

func (s *SQLStore) assignRole(userID int64, roleName string) error {
	var roleID int64
	// #nosec G201 -- placeholders are generated internally for driver compatibility.
	q := `SELECT id FROM roles WHERE name=?`
	if s.dialect == postgresDialect {
		q = `SELECT id FROM roles WHERE name=$1`
	}
	if err := s.db.QueryRow(q, roleName).Scan(&roleID); err != nil {
		return err
	}
	// #nosec G201 -- placeholders are generated internally for driver compatibility.
	insert := fmt.Sprintf(`INSERT INTO user_roles (user_id,role_id) VALUES (%s,%s)`, s.placeholder(1), s.placeholder(2))
	if s.dialect == postgresDialect {
		insert += ` ON CONFLICT (user_id, role_id) DO NOTHING`
	} else {
		insert = strings.Replace(insert, "INSERT INTO", "INSERT OR IGNORE INTO", 1)
	}
	_, err := s.db.Exec(insert, userID, roleID)
	return err
}

func (s *SQLStore) AuthenticateUser(username, password string) (app.User, error) {
	user, err := s.GetUser(username)
	if err != nil {
		return app.User{}, err
	}
	if !user.Enabled {
		return app.User{}, errUserDisabled
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return app.User{}, errInvalidCreds
	}
	return user, nil
}

func (s *SQLStore) GetUser(username string) (app.User, error) {
	// #nosec G201 -- placeholders are generated internally for driver compatibility.
	q := `SELECT id,username,display_name,password_hash,enabled,created_at FROM users WHERE username=?`
	if s.dialect == postgresDialect {
		q = `SELECT id,username,display_name,password_hash,enabled,created_at FROM users WHERE username=$1`
	}
	var user app.User
	if err := s.db.QueryRow(q, username).Scan(&user.ID, &user.Username, &user.DisplayName, &user.PasswordHash, &user.Enabled, &user.CreatedAt); err != nil {
		return app.User{}, err
	}
	roles, err := s.rolesForUser(user.ID)
	if err != nil {
		return app.User{}, err
	}
	user.Roles = roles
	return user, nil
}

func (s *SQLStore) rolesForUser(userID int64) ([]string, error) {
	// #nosec G201 -- placeholders are generated internally for driver compatibility.
	q := `SELECT r.name FROM roles r INNER JOIN user_roles ur ON ur.role_id = r.id WHERE ur.user_id = ? ORDER BY r.name`
	if s.dialect == postgresDialect {
		q = `SELECT r.name FROM roles r INNER JOIN user_roles ur ON ur.role_id = r.id WHERE ur.user_id = $1 ORDER BY r.name`
	}
	rows, err := s.db.Query(q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SQLStore) ListUsers() ([]app.User, error) {
	rows, err := s.db.Query(`SELECT id,username,display_name,password_hash,enabled,created_at FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []app.User
	for rows.Next() {
		var u app.User
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash, &u.Enabled, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.Roles, err = s.rolesForUser(u.ID)
		if err != nil {
			return nil, err
		}
		u.PasswordHash = ""
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *SQLStore) CreateUser(username, displayName, password string, roles []string) (app.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return app.User{}, err
	}
	// #nosec G201 -- placeholders are generated internally for driver compatibility.
	q := fmt.Sprintf(`INSERT INTO users (username,display_name,password_hash,enabled,created_at) VALUES (%s,%s,%s,%s,%s)`, s.placeholder(1), s.placeholder(2), s.placeholder(3), s.placeholder(4), s.placeholder(5))
	if _, err := s.db.Exec(q, username, displayName, string(hash), true, s.nowClock()); err != nil {
		return app.User{}, err
	}
	user, err := s.GetUser(username)
	if err != nil {
		return app.User{}, err
	}
	for _, role := range roles {
		if role == "" {
			continue
		}
		if err := s.assignRole(user.ID, role); err != nil {
			return app.User{}, err
		}
	}
	user, err = s.GetUser(username)
	if err != nil {
		return app.User{}, err
	}
	user.PasswordHash = ""
	return user, nil
}

func (s *SQLStore) ListRoles() ([]app.Role, error) {
	rows, err := s.db.Query(`SELECT id,name,description,permissions,created_at FROM roles ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []app.Role
	for rows.Next() {
		var role app.Role
		var perms string
		if err := rows.Scan(&role.ID, &role.Name, &role.Description, &perms, &role.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(perms), &role.Permissions)
		out = append(out, role)
	}
	return out, rows.Err()
}

func (s *SQLStore) CreateRole(role app.Role) (app.Role, error) {
	role.CreatedAt = s.nowClock()
	perms, err := marshalJSON(role.Permissions)
	if err != nil {
		return app.Role{}, err
	}
	// #nosec G201 -- placeholders are generated internally for driver compatibility.
	q := fmt.Sprintf(`INSERT INTO roles (name,description,permissions,created_at) VALUES (%s,%s,%s,%s)`, s.placeholder(1), s.placeholder(2), s.placeholder(3), s.placeholder(4))
	id, err := s.insertWithID(q, role.Name, role.Description, perms, role.CreatedAt)
	if err != nil {
		return app.Role{}, err
	}
	role.ID = id
	return role, nil
}

func (s *SQLStore) CreateInvite(email, role string) (app.Invite, error) {
	if role == "" {
		role = "viewer"
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return app.Invite{}, err
	}
	invite := app.Invite{Email: email, Role: role, Token: hex.EncodeToString(buf), Status: "pending", CreatedAt: s.nowClock(), ExpiresAt: s.nowClock().Add(7 * 24 * time.Hour)}
	q := fmt.Sprintf(`INSERT INTO invites (email,role_name,token,status,expires_at,created_at) VALUES (%s,%s,%s,%s,%s,%s)`, s.placeholder(1), s.placeholder(2), s.placeholder(3), s.placeholder(4), s.placeholder(5), s.placeholder(6))
	id, err := s.insertWithID(q, invite.Email, invite.Role, invite.Token, invite.Status, invite.ExpiresAt, invite.CreatedAt)
	if err != nil {
		return app.Invite{}, err
	}
	invite.ID = id
	return invite, nil
}

func (s *SQLStore) ListInvites() ([]app.Invite, error) {
	rows, err := s.db.Query(`SELECT id,email,role_name,token,status,expires_at,accepted_at,created_at FROM invites ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []app.Invite
	for rows.Next() {
		var inv app.Invite
		var accepted sql.NullTime
		if err := rows.Scan(&inv.ID, &inv.Email, &inv.Role, &inv.Token, &inv.Status, &inv.ExpiresAt, &accepted, &inv.CreatedAt); err != nil {
			return nil, err
		}
		if accepted.Valid {
			inv.AcceptedAt = &accepted.Time
		}
		out = append(out, inv)
	}
	return out, rows.Err()
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

	q := `INSERT INTO alerts (source,title,description,severity,status,labels,payload,created_at) VALUES (%s,%s,%s,%s,%s,%s,%s,%s)`
	q = fmt.Sprintf(q, s.placeholder(1), s.placeholder(2), s.placeholder(3), s.placeholder(4), s.placeholder(5), s.placeholder(6), s.placeholder(7), s.placeholder(8))
	if a.Status == "" {
		a.Status = "received"
	}
	id, err := s.insertWithID(q, a.Source, a.Title, a.Description, string(a.Severity), a.Status, labelsJSON, payloadJSON, a.CreatedAt)
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

	q := `UPDATE alerts SET triage=%s,status=%s WHERE id=%s`
	q = fmt.Sprintf(q, s.placeholder(1), s.placeholder(2), s.placeholder(3))
	_, err = s.db.Exec(q, triageJSON, "triaged", parseNumericID(alertID))
	return err
}

func (s *SQLStore) UpdateAlertStatus(alertID, status string) error {
	q := `UPDATE alerts SET status=%s WHERE id=%s`
	q = fmt.Sprintf(q, s.placeholder(1), s.placeholder(2))
	_, err := s.db.Exec(q, status, parseNumericID(alertID))
	return err
}

func (s *SQLStore) Alerts() ([]app.Alert, error) {
	rows, err := s.db.Query(`SELECT id,source,title,description,severity,status,labels,payload,triage,created_at FROM alerts ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []app.Alert{}
	for rows.Next() {
		var id int64
		var severity, status, labels, payload string
		var triage sql.NullString
		var a app.Alert
		if err := rows.Scan(&id, &a.Source, &a.Title, &a.Description, &severity, &status, &labels, &payload, &triage, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.ID = fmt.Sprintf("alt-%06d", id)
		a.Severity = app.Severity(severity)
		a.Status = status
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
	if in.CreatedAt.IsZero() {
		in.CreatedAt = s.nowClock()
	}
	if in.Service == "" {
		in.Service = "unknown"
	}
	if in.Status == "resolved" && in.ResolvedAt == nil {
		resolvedAt := s.nowClock()
		in.ResolvedAt = &resolvedAt
	}
	if _, err := s.EnsureService(in.Service); err != nil {
		return app.Incident{}, err
	}
	q := `INSERT INTO incidents (alert_id,service,title,severity,status,status_page_url,created_at,resolved_at) VALUES (%s,%s,%s,%s,%s,%s,%s,%s)`
	q = fmt.Sprintf(q, s.placeholder(1), s.placeholder(2), s.placeholder(3), s.placeholder(4), s.placeholder(5), s.placeholder(6), s.placeholder(7), s.placeholder(8))
	id, err := s.insertWithID(q, in.AlertID, in.Service, in.Title, string(in.Severity), in.Status, in.StatusPageURL, in.CreatedAt, in.ResolvedAt)
	if err != nil {
		return app.Incident{}, err
	}
	in.ID = fmt.Sprintf("inc-%06d", id)
	return in, nil
}

func (s *SQLStore) Incidents() ([]app.Incident, error) {
	rows, err := s.db.Query(`SELECT id,alert_id,service,title,severity,status,status_page_url,created_at,resolved_at FROM incidents ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []app.Incident
	for rows.Next() {
		var id int64
		var sev string
		var in app.Incident
		if err := rows.Scan(&id, &in.AlertID, &in.Service, &in.Title, &sev, &in.Status, &in.StatusPageURL, &in.CreatedAt, &in.ResolvedAt); err != nil {
			return nil, err
		}
		in.ID = fmt.Sprintf("inc-%06d", id)
		in.Severity = app.Severity(sev)
		out = append(out, in)
	}
	return out, rows.Err()
}

func (s *SQLStore) EnsureService(name string) (app.Service, error) {
	if name == "" {
		name = "unknown"
	}
	now := s.nowClock()
	// #nosec G201 -- placeholders are generated internally for driver compatibility.
	q := fmt.Sprintf(`INSERT INTO services (name,description,created_at) VALUES (%s,%s,%s)`, s.placeholder(1), s.placeholder(2), s.placeholder(3))
	if s.dialect == postgresDialect {
		q += ` ON CONFLICT (name) DO NOTHING`
	} else {
		q = strings.Replace(q, "INSERT INTO", "INSERT OR IGNORE INTO", 1)
	}
	if _, err := s.db.Exec(q, name, "", now); err != nil {
		return app.Service{}, err
	}

	lookup := `SELECT id,name,description,created_at FROM services WHERE name=?`
	if s.dialect == postgresDialect {
		lookup = `SELECT id,name,description,created_at FROM services WHERE name=$1`
	}
	var id int64
	var svc app.Service
	if err := s.db.QueryRow(lookup, name).Scan(&id, &svc.Name, &svc.Description, &svc.CreatedAt); err != nil {
		return app.Service{}, err
	}
	svc.ID = fmt.Sprintf("svc-%06d", id)
	return svc, nil
}

func (s *SQLStore) Services() ([]app.Service, error) {
	rows, err := s.db.Query(`SELECT id,name,description,created_at FROM services ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []app.Service
	for rows.Next() {
		var id int64
		var svc app.Service
		if err := rows.Scan(&id, &svc.Name, &svc.Description, &svc.CreatedAt); err != nil {
			return nil, err
		}
		svc.ID = fmt.Sprintf("svc-%06d", id)
		out = append(out, svc)
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

func (s *SQLStore) CreateTool(tool app.MCPTool) (app.MCPTool, error) {
	now := s.nowClock()
	tool.CreatedAt = now
	tool.UpdatedAt = now
	configJSON, err := marshalJSON(tool.Config)
	if err != nil {
		return app.MCPTool{}, err
	}
	q := `INSERT INTO tools (name,description,server,tool,config,created_at,updated_at) VALUES (%s,%s,%s,%s,%s,%s,%s)`
	q = fmt.Sprintf(q, s.placeholder(1), s.placeholder(2), s.placeholder(3), s.placeholder(4), s.placeholder(5), s.placeholder(6), s.placeholder(7))
	id, err := s.insertWithID(q, tool.Name, tool.Description, tool.Server, tool.Tool, configJSON, tool.CreatedAt, tool.UpdatedAt)
	if err != nil {
		return app.MCPTool{}, err
	}
	tool.ID = fmt.Sprintf("tool-%06d", id)
	return tool, nil
}

func (s *SQLStore) Tools() ([]app.MCPTool, error) {
	rows, err := s.db.Query(`SELECT id,name,description,server,tool,config,created_at,updated_at FROM tools ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []app.MCPTool
	for rows.Next() {
		var id int64
		var config string
		var tool app.MCPTool
		if err := rows.Scan(&id, &tool.Name, &tool.Description, &tool.Server, &tool.Tool, &config, &tool.CreatedAt, &tool.UpdatedAt); err != nil {
			return nil, err
		}
		tool.ID = fmt.Sprintf("tool-%06d", id)
		_ = json.Unmarshal([]byte(config), &tool.Config)
		out = append(out, tool)
	}
	return out, rows.Err()
}

func (s *SQLStore) Tool(toolID string) (app.MCPTool, error) {
	q := `SELECT id,name,description,server,tool,config,created_at,updated_at FROM tools WHERE id=?`
	if s.dialect == postgresDialect {
		q = `SELECT id,name,description,server,tool,config,created_at,updated_at FROM tools WHERE id=$1`
	}
	var id int64
	var config string
	var tool app.MCPTool
	if err := s.db.QueryRow(q, parseNumericID(toolID)).Scan(&id, &tool.Name, &tool.Description, &tool.Server, &tool.Tool, &config, &tool.CreatedAt, &tool.UpdatedAt); err != nil {
		return app.MCPTool{}, err
	}
	tool.ID = fmt.Sprintf("tool-%06d", id)
	_ = json.Unmarshal([]byte(config), &tool.Config)
	return tool, nil
}

func (s *SQLStore) UpdateTool(toolID string, tool app.MCPTool) (app.MCPTool, error) {
	tool.UpdatedAt = s.nowClock()
	configJSON, err := marshalJSON(tool.Config)
	if err != nil {
		return app.MCPTool{}, err
	}
	q := `UPDATE tools SET name=%s,description=%s,server=%s,tool=%s,config=%s,updated_at=%s WHERE id=%s`
	q = fmt.Sprintf(q, s.placeholder(1), s.placeholder(2), s.placeholder(3), s.placeholder(4), s.placeholder(5), s.placeholder(6), s.placeholder(7))
	if _, err = s.db.Exec(q, tool.Name, tool.Description, tool.Server, tool.Tool, configJSON, tool.UpdatedAt, parseNumericID(toolID)); err != nil {
		return app.MCPTool{}, err
	}
	stored, err := s.Tool(toolID)
	if err != nil {
		return app.MCPTool{}, err
	}
	return stored, nil
}

func (s *SQLStore) DeleteTool(toolID string) error {
	q := `DELETE FROM tools WHERE id=?`
	if s.dialect == postgresDialect {
		q = `DELETE FROM tools WHERE id=$1`
	}
	_, err := s.db.Exec(q, parseNumericID(toolID))
	return err
}

func parseNumericID(prefixed string) int64 {
	parts := strings.SplitN(prefixed, "-", 2)
	if len(parts) != 2 {
		return 0
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0
	}
	return id
}
