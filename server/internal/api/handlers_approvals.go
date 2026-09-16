package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/southcorner/systemcheck/server/internal/store"
)

const approvalTTL = 8 * time.Hour

type viewRequestReq struct {
	MachineID string `json:"machine_id"`
	Reason    string `json:"reason"`
}

func (a *App) handleCreateViewRequest(w http.ResponseWriter, r *http.Request) {
	var req viewRequestReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.MachineID == "" {
		writeErr(w, http.StatusBadRequest, "machine_id required")
		return
	}
	id, err := a.Store.CreateViewRequest(r.Context(), req.MachineID, currentUser(r).Email, req.Reason, approvalTTL)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store error")
		return
	}
	_ = a.Store.Audit(r.Context(), currentUser(r).Email, "view_request", req.MachineID,
		map[string]interface{}{"reason": req.Reason})
	writeJSON(w, http.StatusOK, map[string]string{"id": id})
}

func (a *App) handleApproveViewRequest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := a.Store.ApproveViewRequest(r.Context(), id, currentUser(r).Email)
	if errors.Is(err, store.ErrSameApprover) {
		writeErr(w, http.StatusForbidden, "a different admin must approve")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "approve error")
		return
	}
	_ = a.Store.Audit(r.Context(), currentUser(r).Email, "view_approve", id, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleListViewRequests(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Store.ListViewRequests(r.Context(), limitParam(r, 200))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query error")
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

// handleAgentVersion advertises the latest agent release for update checks. It
// is read-only and does not require auth (agents call it over mTLS regardless).
func (a *App) handleAgentVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"version":   a.Cfg.AgentLatestVersion,
		"url":       a.Cfg.AgentDownloadURL,
		"signature": a.Cfg.AgentSignatureB64,
	})
}

// handleAgentDownload streams the signed agent binary to enrolled agents over
// their mTLS connection (so no external CDN or public cert is needed). The
// agent verifies the ed25519 signature before installing.
func (a *App) handleAgentDownload(w http.ResponseWriter, r *http.Request) {
	if a.Cfg.AgentBinaryPath == "" {
		writeErr(w, http.StatusNotFound, "no agent binary configured")
		return
	}
	f, err := os.Open(a.Cfg.AgentBinaryPath)
	if err != nil {
		writeErr(w, http.StatusNotFound, "agent binary unavailable")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(w, r, "agent.exe", time.Time{}, f)
}
