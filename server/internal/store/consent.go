package store

import (
	"context"
	"time"
)

// HasConsent reports whether a non-revoked consent record exists for a user.
func (s *Store) HasConsent(ctx context.Context, subjectUser string) (bool, error) {
	var ok bool
	err := s.Pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM consent_records
			 WHERE subject_user=$1 AND revoked_at IS NULL)`, subjectUser).Scan(&ok)
	return ok, err
}

// RecordConsent stores a consent acknowledgement.
func (s *Store) RecordConsent(ctx context.Context, subjectUser, machineID, method string, policyVersion int) error {
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO consent_records(subject_user, machine_id, policy_version, method)
		 VALUES($1, $2, $3, $4)`, subjectUser, nullify(machineID), policyVersion, method)
	return err
}

// RevokeConsent marks all of a user's consent records revoked.
func (s *Store) RevokeConsent(ctx context.Context, subjectUser string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE consent_records SET revoked_at=now()
		  WHERE subject_user=$1 AND revoked_at IS NULL`, subjectUser)
	return err
}

// ConsentRecord is a stored acknowledgement.
type ConsentRecord struct {
	SubjectUser    string     `json:"subject_user"`
	PolicyVersion  int        `json:"policy_version"`
	Method         string     `json:"method"`
	AcknowledgedAt time.Time  `json:"acknowledged_at"`
	RevokedAt      *time.Time `json:"revoked_at"`
}
