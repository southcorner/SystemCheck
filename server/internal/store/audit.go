package store

import (
	"context"
	"time"
)

// Audit appends an entry to the append-only audit log.
func (s *Store) Audit(ctx context.Context, actor, action, target string, detail map[string]interface{}) error {
	if detail == nil {
		detail = map[string]interface{}{}
	}
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO audit_log(actor, action, target, detail) VALUES($1,$2,$3,$4)`,
		actor, action, nullify(target), detail)
	return err
}

// AuditEntry is a stored audit record.
type AuditEntry struct {
	TS     time.Time              `json:"ts"`
	Actor  string                 `json:"actor"`
	Action string                 `json:"action"`
	Target string                 `json:"target"`
	Detail map[string]interface{} `json:"detail"`
}

// ListAudit returns recent audit entries, newest first.
func (s *Store) ListAudit(ctx context.Context, limit int) ([]AuditEntry, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT ts, coalesce(actor,''), action, coalesce(target,''), detail
		   FROM audit_log ORDER BY ts DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.TS, &e.Actor, &e.Action, &e.Target, &e.Detail); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
