package store

import (
	"context"
	"time"
)

// Machine is a monitored endpoint.
type Machine struct {
	ID           string     `json:"id"`
	Hostname     string     `json:"hostname"`
	OS           string     `json:"os"`
	OSVersion    string     `json:"os_version"`
	AssignedUser string     `json:"assigned_user"`
	Group        string     `json:"group"`
	LastSeen     *time.Time `json:"last_seen"`
	AgentVersion string     `json:"agent_version"`
	Active       bool       `json:"active"`
	Nickname     string     `json:"nickname"`
}

// CreateMachine inserts a machine at enrollment time and returns its id.
func (s *Store) CreateMachine(ctx context.Context, hostname, os, osVersion, agentVersion, group, assignedUser, fingerprint string) (string, error) {
	var id string
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO machines(hostname, os, os_version, agent_version, "group", assigned_user, cert_fingerprint, last_seen)
		 VALUES($1,$2,$3,$4,$5,$6,$7, now()) RETURNING id`,
		hostname, os, osVersion, agentVersion, nullify(group), nullify(assignedUser), fingerprint,
	).Scan(&id)
	return id, err
}

// GetMachineByFingerprint resolves a machine by its client-cert fingerprint.
func (s *Store) GetMachineByFingerprint(ctx context.Context, fingerprint string) (*Machine, error) {
	return s.scanMachine(ctx,
		`SELECT id, hostname, os, coalesce(os_version,''), coalesce(assigned_user,''), coalesce("group",''), last_seen, coalesce(agent_version,''), active, coalesce(nickname,'')
		   FROM machines WHERE cert_fingerprint=$1`, fingerprint)
}

// GetMachine resolves a machine by id.
func (s *Store) GetMachine(ctx context.Context, id string) (*Machine, error) {
	return s.scanMachine(ctx,
		`SELECT id, hostname, os, coalesce(os_version,''), coalesce(assigned_user,''), coalesce("group",''), last_seen, coalesce(agent_version,''), active, coalesce(nickname,'')
		   FROM machines WHERE id=$1`, id)
}

func (s *Store) scanMachine(ctx context.Context, q string, args ...any) (*Machine, error) {
	m := &Machine{}
	err := s.Pool.QueryRow(ctx, q, args...).Scan(
		&m.ID, &m.Hostname, &m.OS, &m.OSVersion, &m.AssignedUser, &m.Group, &m.LastSeen, &m.AgentVersion, &m.Active, &m.Nickname)
	if err != nil {
		return nil, noRows(err)
	}
	return m, nil
}

// ListMachines returns all machines, most-recently-seen first.
func (s *Store) ListMachines(ctx context.Context) ([]Machine, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, hostname, os, coalesce(os_version,''), coalesce(assigned_user,''), coalesce("group",''), last_seen, coalesce(agent_version,''), active, coalesce(nickname,'')
		   FROM machines ORDER BY last_seen DESC NULLS LAST`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Machine
	for rows.Next() {
		var m Machine
		if err := rows.Scan(&m.ID, &m.Hostname, &m.OS, &m.OSVersion, &m.AssignedUser, &m.Group, &m.LastSeen, &m.AgentVersion, &m.Active, &m.Nickname); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SetMachineNickname sets (or clears) the human-friendly label for a machine.
func (s *Store) SetMachineNickname(ctx context.Context, id, nickname string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE machines SET nickname=$2 WHERE id=$1`, id, nullify(nickname))
	return err
}

// TouchMachine updates last_seen and agent version on heartbeat.
func (s *Store) TouchMachine(ctx context.Context, id, agentVersion string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE machines SET last_seen=now(), agent_version=$2 WHERE id=$1`, id, agentVersion)
	return err
}

// SetMachineAssignedUser attributes a machine to a real interactive user. Used
// when the session helper reports the logged-in identity, so per-person
// attribution and the consent gate track the actual employee.
func (s *Store) SetMachineAssignedUser(ctx context.Context, id, assignedUser string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE machines SET assigned_user=$2 WHERE id=$1`, id, nullify(assignedUser))
	return err
}

// --- Enrollment tokens ---

// CreateEnrollmentToken stores a hashed one-time token.
func (s *Store) CreateEnrollmentToken(ctx context.Context, tokenHash, group, assignedUser string, expires time.Time) error {
	return s.CreateEnrollmentTokenEx(ctx, tokenHash, group, assignedUser, expires, false, 0)
}

// CreateEnrollmentTokenEx stores a hashed token that may be reusable. When
// reusable is true the token enrolls up to maxUses machines within its TTL
// (maxUses <= 0 means unlimited); when false it is single-use.
func (s *Store) CreateEnrollmentTokenEx(ctx context.Context, tokenHash, group, assignedUser string, expires time.Time, reusable bool, maxUses int) error {
	var mu any
	if reusable && maxUses > 0 {
		mu = maxUses
	}
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO enrollment_tokens(token_hash, "group", assigned_user, expires_at, reusable, max_uses)
		 VALUES($1,$2,$3,$4,$5,$6)`, tokenHash, nullify(group), nullify(assignedUser), expires, reusable, mu)
	return err
}

// EnrollmentToken is a resolved (unused, unexpired) enrollment token.
type EnrollmentToken struct {
	Group        string
	AssignedUser string
}

// ConsumeEnrollmentToken atomically validates and consumes a token. A
// single-use token (reusable = false) is valid only while unused. A reusable
// token is valid while not expired and under its max_uses cap (NULL = no cap).
// Every successful consume increments use_count and stamps used_at on first use.
func (s *Store) ConsumeEnrollmentToken(ctx context.Context, tokenHash string) (*EnrollmentToken, error) {
	t := &EnrollmentToken{}
	var group, user *string
	err := s.Pool.QueryRow(ctx,
		`UPDATE enrollment_tokens
		    SET use_count = use_count + 1,
		        used_at   = COALESCE(used_at, now())
		  WHERE token_hash = $1
		    AND expires_at > now()
		    AND ( (NOT reusable AND used_at IS NULL)
		          OR (reusable AND (max_uses IS NULL OR use_count < max_uses)) )
		 RETURNING "group", assigned_user`, tokenHash,
	).Scan(&group, &user)
	if err != nil {
		return nil, noRows(err)
	}
	if group != nil {
		t.Group = *group
	}
	if user != nil {
		t.AssignedUser = *user
	}
	return t, nil
}

func nullify(s string) any {
	if s == "" {
		return nil
	}
	return s
}
