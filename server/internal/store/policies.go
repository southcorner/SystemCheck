package store

import (
	"context"
	"encoding/json"

	"github.com/southcorner/systemcheck/server/internal/model"
)

// EffectivePolicy resolves the policy for a machine: a machine-specific
// assignment wins, then a group assignment, else the built-in default. The
// returned policy's Active flag additionally requires a consent record for the
// machine's assigned user (privacy gate).
func (s *Store) EffectivePolicy(ctx context.Context, m *Machine) (model.Policy, error) {
	pol := model.DefaultPolicy()

	var doc []byte
	var version int
	err := s.Pool.QueryRow(ctx,
		`SELECT p.doc, p.version
		   FROM policy_assignments a JOIN policies p ON p.id = a.policy_id
		  WHERE a.machine_id = $1
		  ORDER BY p.version DESC LIMIT 1`, m.ID).Scan(&doc, &version)
	if err != nil && m.Group != "" {
		err = s.Pool.QueryRow(ctx,
			`SELECT p.doc, p.version
			   FROM policy_assignments a JOIN policies p ON p.id = a.policy_id
			  WHERE a.machine_id IS NULL AND a."group" = $1
			  ORDER BY p.version DESC LIMIT 1`, m.Group).Scan(&doc, &version)
	}
	if err == nil && len(doc) > 0 {
		if uErr := json.Unmarshal(doc, &pol); uErr != nil {
			return pol, uErr
		}
		pol.Version = version
	}

	// Consent gate.
	pol.Active = false
	if m.AssignedUser != "" {
		ok, cErr := s.HasConsent(ctx, m.AssignedUser)
		if cErr != nil {
			return pol, cErr
		}
		pol.Active = ok
	}
	return pol, nil
}

// GetGroupPolicy returns the policy currently assigned to a group (for the
// Settings UI to edit), or the built-in default (version 0) if none is set.
func (s *Store) GetGroupPolicy(ctx context.Context, group string) (model.Policy, int, error) {
	pol := model.DefaultPolicy()
	var doc []byte
	var version int
	err := s.Pool.QueryRow(ctx,
		`SELECT p.doc, p.version
		   FROM policy_assignments a JOIN policies p ON p.id = a.policy_id
		  WHERE a.machine_id IS NULL AND a."group" = $1
		  ORDER BY p.version DESC LIMIT 1`, group).Scan(&doc, &version)
	if err == nil && len(doc) > 0 {
		if uErr := json.Unmarshal(doc, &pol); uErr != nil {
			return pol, 0, uErr
		}
		pol.Version = version
		return pol, version, nil
	}
	return pol, 0, nil
}

// SetGroupPolicy stores a new policy version for a group and repoints the
// group's assignment to it, atomically. Bumping the version is what makes
// agents pick up the change on their next poll.
func (s *Store) SetGroupPolicy(ctx context.Context, group string, doc model.Policy) (int, error) {
	raw, err := json.Marshal(doc)
	if err != nil {
		return 0, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var id string
	var version int
	if err := tx.QueryRow(ctx,
		`INSERT INTO policies(name, version, doc)
		 VALUES($1, COALESCE((SELECT max(version) FROM policies WHERE name=$1),0)+1, $2)
		 RETURNING id, version`, group, raw).Scan(&id, &version); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM policy_assignments WHERE machine_id IS NULL AND "group" = $1`, group); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO policy_assignments(machine_id, "group", policy_id) VALUES(NULL, $1, $2)`, group, id); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return version, nil
}

// UpsertPolicy stores a policy document under a name, bumping the version.
func (s *Store) UpsertPolicy(ctx context.Context, name string, doc model.Policy) (string, int, error) {
	raw, err := json.Marshal(doc)
	if err != nil {
		return "", 0, err
	}
	var id string
	var version int
	// New version = max existing for this name + 1.
	err = s.Pool.QueryRow(ctx,
		`INSERT INTO policies(name, version, doc)
		 VALUES($1, COALESCE((SELECT max(version) FROM policies WHERE name=$1),0)+1, $2)
		 RETURNING id, version`, name, raw).Scan(&id, &version)
	return id, version, err
}
