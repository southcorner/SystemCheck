package store

import (
	"context"
	"time"
)

// AlertRule is a configured detection rule.
type AlertRule struct {
	ID      string                 `json:"id"`
	Name    string                 `json:"name"`
	Kind    string                 `json:"kind"` // domain_blocklist|large_upload|app_blocklist
	Params  map[string]interface{} `json:"params"`
	Enabled bool                   `json:"enabled"`
}

// ListAlertRules returns all enabled rules (or all when includeDisabled).
func (s *Store) ListAlertRules(ctx context.Context, includeDisabled bool) ([]AlertRule, error) {
	q := `SELECT id, name, kind, params, enabled FROM alert_rules`
	if !includeDisabled {
		q += ` WHERE enabled = TRUE`
	}
	rows, err := s.Pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AlertRule
	for rows.Next() {
		var r AlertRule
		if err := rows.Scan(&r.ID, &r.Name, &r.Kind, &r.Params, &r.Enabled); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CreateAlertRule inserts a rule and returns its id.
func (s *Store) CreateAlertRule(ctx context.Context, name, kind string, params map[string]interface{}) (string, error) {
	if params == nil {
		params = map[string]interface{}{}
	}
	var id string
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO alert_rules(name, kind, params) VALUES($1,$2,$3) RETURNING id`,
		name, kind, params).Scan(&id)
	return id, err
}

// CreateAlert records a fired alert.
func (s *Store) CreateAlert(ctx context.Context, ruleID, machineID, severity, message string, data map[string]interface{}) error {
	if data == nil {
		data = map[string]interface{}{}
	}
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO alerts(rule_id, machine_id, severity, message, data)
		 VALUES($1,$2,$3,$4,$5)`,
		nullify(ruleID), nullify(machineID), severity, message, data)
	return err
}

// Alert is a fired alert row.
type Alert struct {
	ID           string                 `json:"id"`
	MachineID    string                 `json:"machine_id"`
	TS           time.Time              `json:"ts"`
	Severity     string                 `json:"severity"`
	Message      string                 `json:"message"`
	Data         map[string]interface{} `json:"data"`
	Acknowledged bool                   `json:"acknowledged"`
}

// ListAlerts returns recent alerts, newest first. When unackedOnly, only
// unacknowledged alerts are returned.
func (s *Store) ListAlerts(ctx context.Context, unackedOnly bool, limit int) ([]Alert, error) {
	q := `SELECT id, coalesce(machine_id::text,''), ts, severity, message, data, acknowledged FROM alerts`
	if unackedOnly {
		q += ` WHERE acknowledged = FALSE`
	}
	q += ` ORDER BY ts DESC LIMIT $1`
	rows, err := s.Pool.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Alert
	for rows.Next() {
		var a Alert
		if err := rows.Scan(&a.ID, &a.MachineID, &a.TS, &a.Severity, &a.Message, &a.Data, &a.Acknowledged); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AckAlert marks an alert acknowledged.
func (s *Store) AckAlert(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE alerts SET acknowledged=TRUE WHERE id=$1`, id)
	return err
}
