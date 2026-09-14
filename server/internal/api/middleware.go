package api

import (
	"net/http"
	"strings"

	"github.com/southcorner/systemcheck/server/internal/auth"
	"github.com/southcorner/systemcheck/server/internal/store"
)

// sessionToken extracts the session token from the Authorization bearer header
// or the sc_session cookie.
func sessionToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	if c, err := r.Cookie("sc_session"); err == nil {
		return c.Value
	}
	return ""
}

// adminAuth requires a valid admin session. If requireMFA is true, the session
// must have completed MFA.
func (a *App) adminAuth(requireMFA bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		if requireMFA && !sess.MFAPassed {
			writeErr(w, http.StatusForbidden, "mfa required")
			return
		}
		user, err := a.Store.GetAdminByID(r.Context(), sess.UserID)
		if err != nil || user.Disabled {
			writeErr(w, http.StatusUnauthorized, "invalid user")
			return
		}
		ctx := ctxWith(r.Context(), ctxUser, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireRole restricts access to the given roles (pass "" for the second to
// require only the first).
func (a *App) requireRole(r1, r2 string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := r.Context().Value(ctxUser).(*store.AdminUser)
		if user == nil {
			writeErr(w, http.StatusUnauthorized, "no user")
			return
		}
		role := string(user.Role)
		if role == r1 || (r2 != "" && role == r2) {
			next.ServeHTTP(w, r)
			return
		}
		writeErr(w, http.StatusForbidden, "insufficient role")
	})
}

// agentAuth requires a valid enrolled client certificate and loads the machine.
func (a *App) agentAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
			writeErr(w, http.StatusUnauthorized, "client certificate required")
			return
		}
		fp := fingerprintOf(r)
		m, err := a.Store.GetMachineByFingerprint(r.Context(), fp)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "unknown or unenrolled agent")
			return
		}
		if !m.Active {
			writeErr(w, http.StatusForbidden, "agent disabled")
			return
		}
		ctx := ctxWith(r.Context(), ctxMachine, m)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
