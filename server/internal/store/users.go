package store

import (
	"context"
	"time"

	"github.com/southcorner/systemcheck/server/internal/model"
)

// AdminUser is an administrator account.
type AdminUser struct {
	ID           string
	Email        string
	PasswordHash string
	Role         model.Role
	TOTPSecret   string
	MFAEnabled   bool
	Disabled     bool
}

// CountAdmins returns the number of admin accounts.
func (s *Store) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM admin_users`).Scan(&n)
	return n, err
}

// CreateAdmin inserts a new admin and returns its id.
func (s *Store) CreateAdmin(ctx context.Context, email, passwordHash string, role model.Role) (string, error) {
	var id string
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO admin_users(email, password_hash, role) VALUES($1,$2,$3) RETURNING id`,
		email, passwordHash, string(role),
	).Scan(&id)
	return id, err
}

// GetAdminByEmail loads an admin by email.
func (s *Store) GetAdminByEmail(ctx context.Context, email string) (*AdminUser, error) {
	u := &AdminUser{}
	var role string
	var totp *string
	err := s.Pool.QueryRow(ctx,
		`SELECT id, email, password_hash, role, totp_secret, mfa_enabled, disabled
		   FROM admin_users WHERE email=$1`, email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &role, &totp, &u.MFAEnabled, &u.Disabled)
	if err != nil {
		return nil, noRows(err)
	}
	u.Role = model.Role(role)
	if totp != nil {
		u.TOTPSecret = *totp
	}
	return u, nil
}

// GetAdminByID loads an admin by id.
func (s *Store) GetAdminByID(ctx context.Context, id string) (*AdminUser, error) {
	u := &AdminUser{}
	var role string
	var totp *string
	err := s.Pool.QueryRow(ctx,
		`SELECT id, email, password_hash, role, totp_secret, mfa_enabled, disabled
		   FROM admin_users WHERE id=$1`, id,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &role, &totp, &u.MFAEnabled, &u.Disabled)
	if err != nil {
		return nil, noRows(err)
	}
	u.Role = model.Role(role)
	if totp != nil {
		u.TOTPSecret = *totp
	}
	return u, nil
}

// AdminSummary is a non-sensitive admin listing row.
type AdminSummary struct {
	ID         string     `json:"id"`
	Email      string     `json:"email"`
	Role       model.Role `json:"role"`
	MFAEnabled bool       `json:"mfa_enabled"`
	Disabled   bool       `json:"disabled"`
}

// ListAdmins returns all admin accounts (no secrets).
func (s *Store) ListAdmins(ctx context.Context) ([]AdminSummary, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, email, role, mfa_enabled, disabled FROM admin_users ORDER BY email`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminSummary
	for rows.Next() {
		var a AdminSummary
		var role string
		if err := rows.Scan(&a.ID, &a.Email, &role, &a.MFAEnabled, &a.Disabled); err != nil {
			return nil, err
		}
		a.Role = model.Role(role)
		out = append(out, a)
	}
	return out, rows.Err()
}

// CountEnabledAdmins counts non-disabled users with the admin role.
func (s *Store) CountEnabledAdmins(ctx context.Context) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx,
		`SELECT count(*) FROM admin_users WHERE role='admin' AND disabled=FALSE`).Scan(&n)
	return n, err
}

// SetRole updates a user's role.
func (s *Store) SetRole(ctx context.Context, userID string, role model.Role) error {
	_, err := s.Pool.Exec(ctx, `UPDATE admin_users SET role=$2 WHERE id=$1`, userID, string(role))
	return err
}

// SetDisabled enables/disables a user account.
func (s *Store) SetDisabled(ctx context.Context, userID string, disabled bool) error {
	_, err := s.Pool.Exec(ctx, `UPDATE admin_users SET disabled=$2 WHERE id=$1`, userID, disabled)
	return err
}

// SetTOTPSecret stores a (not yet enabled) TOTP secret for a user.
func (s *Store) SetTOTPSecret(ctx context.Context, userID, secret string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE admin_users SET totp_secret=$2, mfa_enabled=FALSE WHERE id=$1`, userID, secret)
	return err
}

// EnableMFA marks MFA active for a user.
func (s *Store) EnableMFA(ctx context.Context, userID string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE admin_users SET mfa_enabled=TRUE WHERE id=$1`, userID)
	return err
}

// --- Sessions ---

// CreateSession stores a session token hash and returns nothing extra.
func (s *Store) CreateSession(ctx context.Context, userID, tokenHash string, expires time.Time, mfaPassed bool) error {
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO sessions(user_id, token_hash, expires_at, mfa_passed) VALUES($1,$2,$3,$4)`,
		userID, tokenHash, expires, mfaPassed)
	return err
}

// Session is a resolved login session.
type Session struct {
	ID        string
	UserID    string
	MFAPassed bool
	ExpiresAt time.Time
}

// GetSessionByToken resolves a session from its token hash, if unexpired.
func (s *Store) GetSessionByToken(ctx context.Context, tokenHash string) (*Session, error) {
	sess := &Session{}
	err := s.Pool.QueryRow(ctx,
		`SELECT id, user_id, mfa_passed, expires_at FROM sessions
		  WHERE token_hash=$1 AND expires_at > now()`, tokenHash,
	).Scan(&sess.ID, &sess.UserID, &sess.MFAPassed, &sess.ExpiresAt)
	if err != nil {
		return nil, noRows(err)
	}
	return sess, nil
}

// MarkSessionMFAPassed flips a session to MFA-verified.
func (s *Store) MarkSessionMFAPassed(ctx context.Context, tokenHash string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE sessions SET mfa_passed=TRUE WHERE token_hash=$1`, tokenHash)
	return err
}

// DeleteSession removes a session (logout).
func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash=$1`, tokenHash)
	return err
}
