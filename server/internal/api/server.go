// Package api wires HTTP routing, middleware, and handlers.
package api

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/southcorner/systemcheck/server/internal/blob"
	"github.com/southcorner/systemcheck/server/internal/config"
	"github.com/southcorner/systemcheck/server/internal/notify"
	"github.com/southcorner/systemcheck/server/internal/pki"
	"github.com/southcorner/systemcheck/server/internal/store"
)

// App holds server dependencies shared across handlers.
type App struct {
	Cfg      *config.Config
	Store    *store.Store
	Blob     blob.Store
	CA       *pki.CA
	Notifier *notify.Webhook
	Log      *log.Logger
}

// New constructs an App.
func New(cfg *config.Config, st *store.Store, bl blob.Store, ca *pki.CA) *App {
	return &App{
		Cfg: cfg, Store: st, Blob: bl, CA: ca,
		Notifier: notify.NewWebhook(cfg.AlertWebhook),
		Log:      log.New(os.Stdout, "api ", log.LstdFlags),
	}
}

// Router builds the HTTP handler.
func (a *App) Router() http.Handler {
	mux := http.NewServeMux()

	// Agent endpoints (mTLS for all but enroll).
	mux.HandleFunc("POST /v1/enroll", a.handleEnroll)
	mux.Handle("GET /v1/agent-version", a.agentAuth(http.HandlerFunc(a.handleAgentVersion)))
	mux.Handle("GET /v1/policy", a.agentAuth(http.HandlerFunc(a.handlePolicy)))
	mux.Handle("POST /v1/ingest", a.agentAuth(http.HandlerFunc(a.handleIngest)))
	mux.Handle("POST /v1/screenshots", a.agentAuth(http.HandlerFunc(a.handleScreenshotUpload)))
	mux.Handle("POST /v1/heartbeat", a.agentAuth(http.HandlerFunc(a.handleHeartbeat)))

	// Admin auth.
	mux.HandleFunc("POST /api/login", a.handleLogin)
	mux.HandleFunc("POST /api/mfa/verify", a.handleMFAVerify)
	mux.Handle("POST /api/mfa/enroll", a.adminAuth(false, http.HandlerFunc(a.handleMFAEnroll)))
	mux.Handle("POST /api/logout", a.adminAuth(false, http.HandlerFunc(a.handleLogout)))
	mux.Handle("GET /api/me", a.adminAuth(false, http.HandlerFunc(a.handleMe)))

	// Admin data (require MFA-complete session).
	mux.Handle("GET /api/machines", a.adminAuth(true, http.HandlerFunc(a.handleListMachines)))
	mux.Handle("POST /api/machines/{id}/nickname", a.adminAuth(true, a.requireRole("admin", "", http.HandlerFunc(a.handleSetNickname))))
	mux.Handle("GET /api/machines/{id}/screenshots", a.adminAuth(true, http.HandlerFunc(a.handleMachineScreenshots)))
	mux.Handle("GET /api/machines/{id}/apps", a.adminAuth(true, http.HandlerFunc(a.handleMachineApps)))
	mux.Handle("GET /api/machines/{id}/events", a.adminAuth(true, http.HandlerFunc(a.handleMachineEvents)))
	mux.Handle("GET /api/machines/{id}/domains", a.adminAuth(true, http.HandlerFunc(a.handleMachineDomains)))
	mux.Handle("GET /api/machines/{id}/visits", a.adminAuth(true, http.HandlerFunc(a.handleMachineVisits)))
	mux.Handle("GET /api/machines/{id}/transfers", a.adminAuth(true, http.HandlerFunc(a.handleMachineTransfers)))
	mux.Handle("GET /api/machines/{id}/downloads", a.adminAuth(true, http.HandlerFunc(a.handleMachineDownloads)))
	mux.Handle("GET /api/screenshots/{id}/image", a.adminAuth(true, http.HandlerFunc(a.handleScreenshotImage)))
	mux.Handle("DELETE /api/screenshots/{id}", a.adminAuth(true, a.requireRole("admin", "", http.HandlerFunc(a.handleDeleteScreenshot))))
	mux.Handle("POST /api/screenshots/delete", a.adminAuth(true, a.requireRole("admin", "", http.HandlerFunc(a.handleBulkDeleteScreenshots))))
	mux.Handle("GET /api/audit", a.adminAuth(true, a.requireRole("admin", "auditor", http.HandlerFunc(a.handleAudit))))

	// Alerts.
	mux.Handle("GET /api/alerts", a.adminAuth(true, http.HandlerFunc(a.handleListAlerts)))
	mux.Handle("POST /api/alerts/{id}/ack", a.adminAuth(true, http.HandlerFunc(a.handleAckAlert)))
	mux.Handle("GET /api/alert-rules", a.adminAuth(true, http.HandlerFunc(a.handleListAlertRules)))
	mux.Handle("POST /api/alert-rules", a.adminAuth(true, a.requireRole("admin", "", http.HandlerFunc(a.handleCreateAlertRule))))

	// Admin user management (admin only).
	mux.Handle("GET /api/users", a.adminAuth(true, a.requireRole("admin", "", http.HandlerFunc(a.handleListUsers))))
	mux.Handle("POST /api/users", a.adminAuth(true, a.requireRole("admin", "", http.HandlerFunc(a.handleCreateUser))))
	mux.Handle("POST /api/users/{id}/role", a.adminAuth(true, a.requireRole("admin", "", http.HandlerFunc(a.handleSetUserRole))))
	mux.Handle("POST /api/users/{id}/disable", a.adminAuth(true, a.requireRole("admin", "", http.HandlerFunc(a.handleDisableUser))))

	// Dual-approval view requests.
	mux.Handle("GET /api/view-requests", a.adminAuth(true, http.HandlerFunc(a.handleListViewRequests)))
	mux.Handle("POST /api/view-requests", a.adminAuth(true, http.HandlerFunc(a.handleCreateViewRequest)))
	mux.Handle("POST /api/view-requests/{id}/approve", a.adminAuth(true, http.HandlerFunc(a.handleApproveViewRequest)))

	// Enrollment token minting + consent + policy management (admin only).
	mux.Handle("POST /api/enroll-tokens", a.adminAuth(true, a.requireRole("admin", "", http.HandlerFunc(a.handleCreateEnrollToken))))
	mux.Handle("POST /api/consent", a.adminAuth(true, a.requireRole("admin", "", http.HandlerFunc(a.handleRecordConsent))))
	mux.Handle("POST /api/policies", a.adminAuth(true, a.requireRole("admin", "", http.HandlerFunc(a.handleUpsertPolicy))))
	mux.Handle("GET /api/policy", a.adminAuth(true, http.HandlerFunc(a.handleGetPolicy)))
	mux.Handle("PUT /api/policy", a.adminAuth(true, a.requireRole("admin", "", http.HandlerFunc(a.handleSetPolicy))))

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })

	return logging(a.Log, mux)
}

// TLSConfig returns a config that requests (but does not require) client certs,
// so /v1/enroll works without one while agent endpoints can verify them.
func (a *App) TLSConfig() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(a.Cfg.TLSCert, a.Cfg.TLSKey)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	pool.AddCert(mustParse(a.CA.CAPEM()))
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.VerifyClientCertIfGiven,
		ClientCAs:    pool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func mustParse(caPEM []byte) *x509.Certificate {
	block, _ := pem.Decode(caPEM)
	if block == nil {
		return nil
	}
	c, _ := x509.ParseCertificate(block.Bytes)
	return c
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func logging(l *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		l.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// contextKey is unexported to avoid collisions.
type contextKey string

const (
	ctxUser    contextKey = "user"
	ctxMachine contextKey = "machine"
)

func ctxWith(ctx context.Context, k contextKey, v any) context.Context {
	return context.WithValue(ctx, k, v)
}

func currentUser(r *http.Request) *store.AdminUser {
	u, _ := r.Context().Value(ctxUser).(*store.AdminUser)
	return u
}

func currentMachine(r *http.Request) *store.Machine {
	m, _ := r.Context().Value(ctxMachine).(*store.Machine)
	return m
}

func fingerprintOf(r *http.Request) string {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return ""
	}
	return pki.Fingerprint(r.TLS.PeerCertificates[0])
}
