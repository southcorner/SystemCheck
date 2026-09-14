package store

import (
	"context"
	"errors"
	"time"
)

// ErrSameApprover is returned when an admin tries to approve their own request.
var ErrSameApprover = errors.New("approval must be granted by a different admin")

// ViewRequest is a request/approval to view a machine's screenshots.
type ViewRequest struct {
	ID          string     `json:"id"`
	MachineID   string     `json:"machine_id"`
	RequestedBy string     `json:"requested_by"`
	Reason      string     `json:"reason"`
	ApprovedBy  string     `json:"approved_by"`
	CreatedAt   time.Time  `json:"created_at"`
	ApprovedAt  *time.Time `json:"approved_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
}

// CreateViewRequest records a pending request and returns its id.
func (s *Store) CreateViewRequest(ctx context.Context, machineID, requestedBy, reason string, ttl time.Duration) (string, error) {
	var id string
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO view_approvals(machine_id, requested_by, reason, expires_at)
		 VALUES($1,$2,$3,$4) RETURNING id`,
		machineID, requestedBy, nullify(reason), time.Now().Add(ttl)).Scan(&id)
	return id, err
}

// ApproveViewRequest marks a request approved by a different admin. Returns
// ErrSameApprover if approver == requester.
func (s *Store) ApproveViewRequest(ctx context.Context, id, approvedBy string) error {
	var requestedBy string
	if err := s.Pool.QueryRow(ctx,
		`SELECT requested_by FROM view_approvals WHERE id=$1`, id).Scan(&requestedBy); err != nil {
		return noRows(err)
	}
	if requestedBy == approvedBy {
		return ErrSameApprover
	}
	_, err := s.Pool.Exec(ctx,
		`UPDATE view_approvals SET approved_by=$2, approved_at=now() WHERE id=$1`, id, approvedBy)
	return err
}

// HasActiveApproval reports whether an approved, unexpired approval exists for
// (user, machine).
func (s *Store) HasActiveApproval(ctx context.Context, user, machineID string) (bool, error) {
	var ok bool
	err := s.Pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM view_approvals
			 WHERE machine_id=$1 AND requested_by=$2
			   AND approved_by IS NOT NULL AND expires_at > now())`,
		machineID, user).Scan(&ok)
	return ok, err
}

// ListViewRequests returns recent view requests, newest first.
func (s *Store) ListViewRequests(ctx context.Context, limit int) ([]ViewRequest, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, machine_id, requested_by, coalesce(reason,''), coalesce(approved_by,''),
		        created_at, approved_at, expires_at
		   FROM view_approvals ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ViewRequest
	for rows.Next() {
		var v ViewRequest
		if err := rows.Scan(&v.ID, &v.MachineID, &v.RequestedBy, &v.Reason, &v.ApprovedBy,
			&v.CreatedAt, &v.ApprovedAt, &v.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
