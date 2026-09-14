package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/southcorner/systemcheck/server/internal/auth"
)

const sessionTTL = 12 * time.Hour

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResp struct {
	Token       string `json:"token"`
	MFARequired bool   `json:"mfa_required"`
	MFAEnrolled bool   `json:"mfa_enrolled"`
	Role        string `json:"role"`
}

// handleLogin verifies credentials and issues a session token. If MFA is
// enabled, the session is not MFA-passed until /api/mfa/verify succeeds.
func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad request")
		return
	}
	user, err := a.Store.GetAdminByEmail(r.Context(), req.Email)
	if err != nil || user.Disabled {
		writeErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	ok, err := auth.VerifyPassword(req.Password, user.PasswordHash)
	if err != nil || !ok {
		_ = a.Store.Audit(r.Context(), req.Email, "login_failed", "", nil)
		writeErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	tok, err := auth.NewToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "token error")
		return
	}
	// MFA passes immediately only if the user has not enrolled MFA yet
	// (so they can reach the enrollment endpoint); once enrolled it's required.
	mfaPassed := !user.MFAEnabled
	if err := a.Store.CreateSession(r.Context(), user.ID, auth.HashToken(tok), time.Now().Add(sessionTTL), mfaPassed); err != nil {
		writeErr(w, http.StatusInternalServerError, "session error")
		return
	}
	_ = a.Store.Audit(r.Context(), user.Email, "login", "", nil)
	setSessionCookie(w, tok)
	writeJSON(w, http.StatusOK, loginResp{
		Token: tok, MFARequired: user.MFAEnabled, MFAEnrolled: user.MFAEnabled, Role: string(user.Role),
	})
}

type mfaVerifyReq struct {
	Code string `json:"code"`
}

// handleMFAVerify checks a TOTP code and marks the session MFA-passed.
func (a *App) handleMFAVerify(w http.ResponseWriter, r *http.Request) {
	tok := sessionToken(r)
	if tok == "" {
		writeErr(w, http.StatusUnauthorized, "missing session")
		return
	}
	sess, err := a.Store.GetSessionByToken(r.Context(), auth.HashToken(tok))
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid session")
		return
	}
	user, err := a.Store.GetAdminByID(r.Context(), sess.UserID)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid user")
		return
	}
	var req mfaVerifyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad request")
		return
	}
	if user.TOTPSecret == "" || !auth.VerifyTOTP(user.TOTPSecret, req.Code) {
		_ = a.Store.Audit(r.Context(), user.Email, "mfa_failed", "", nil)
		writeErr(w, http.StatusForbidden, "invalid code")
		return
	}
	if err := a.Store.MarkSessionMFAPassed(r.Context(), auth.HashToken(tok)); err != nil {
		writeErr(w, http.StatusInternalServerError, "session error")
		return
	}
	if !user.MFAEnabled {
		_ = a.Store.EnableMFA(r.Context(), user.ID)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type mfaEnrollResp struct {
	Secret string `json:"secret"`
	URI    string `json:"uri"`
}

// handleMFAEnroll generates and stores a TOTP secret (not yet enabled) and
// returns the provisioning URI. MFA activates on first successful verify.
func (a *App) handleMFAEnroll(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	secret, err := auth.NewTOTPSecret()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "secret error")
		return
	}
	if err := a.Store.SetTOTPSecret(r.Context(), user.ID, secret); err != nil {
		writeErr(w, http.StatusInternalServerError, "store error")
		return
	}
	writeJSON(w, http.StatusOK, mfaEnrollResp{
		Secret: secret,
		URI:    auth.TOTPURI("SystemCheck", user.Email, secret),
	})
}

// handleLogout deletes the current session.
func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	tok := sessionToken(r)
	if tok != "" {
		_ = a.Store.DeleteSession(r.Context(), auth.HashToken(tok))
	}
	clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleMe returns the current admin identity.
func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"email":       user.Email,
		"role":        user.Role,
		"mfa_enabled": user.MFAEnabled,
	})
}

func setSessionCookie(w http.ResponseWriter, tok string) {
	http.SetCookie(w, &http.Cookie{
		Name: "sc_session", Value: tok, Path: "/",
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
		Expires: time.Now().Add(sessionTTL),
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: "sc_session", Value: "", Path: "/",
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode, MaxAge: -1,
	})
}
