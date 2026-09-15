package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/southcorner/systemcheck/server/internal/auth"
	"github.com/southcorner/systemcheck/server/internal/model"
)

type enrollTokenReq struct {
	Group        string `json:"group"`
	AssignedUser string `json:"assigned_user"`
	TTLMinutes   int    `json:"ttl_minutes"`
	Reusable     bool   `json:"reusable"`  // if true, the token may enroll many machines
	MaxUses      int    `json:"max_uses"`  // cap on reusable enrollments; <=0 means unlimited
}

type enrollTokenResp struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	Reusable  bool      `json:"reusable"`
	MaxUses   int       `json:"max_uses"`
}

// handleCreateEnrollToken mints a one-time enrollment token (shown once).
func (a *App) handleCreateEnrollToken(w http.ResponseWriter, r *http.Request) {
	var req enrollTokenReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad request")
		return
	}
	ttl := time.Duration(req.TTLMinutes) * time.Minute
	if ttl <= 0 {
		ttl = 60 * time.Minute
	}
	tok, err := auth.NewToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "token error")
		return
	}
	expires := time.Now().Add(ttl)
	if err := a.Store.CreateEnrollmentTokenEx(r.Context(), auth.HashToken(tok), req.Group, req.AssignedUser, expires, req.Reusable, req.MaxUses); err != nil {
		writeErr(w, http.StatusInternalServerError, "store error")
		return
	}
	_ = a.Store.Audit(r.Context(), currentUser(r).Email, "create_enroll_token", req.AssignedUser,
		map[string]interface{}{"group": req.Group, "reusable": req.Reusable, "max_uses": req.MaxUses})
	writeJSON(w, http.StatusOK, enrollTokenResp{Token: tok, ExpiresAt: expires, Reusable: req.Reusable, MaxUses: req.MaxUses})
}

type consentReq struct {
	SubjectUser   string `json:"subject_user"`
	MachineID     string `json:"machine_id"`
	Method        string `json:"method"`
	PolicyVersion int    `json:"policy_version"`
}

// handleRecordConsent records a consent acknowledgement that gates monitoring.
func (a *App) handleRecordConsent(w http.ResponseWriter, r *http.Request) {
	var req consentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SubjectUser == "" {
		writeErr(w, http.StatusBadRequest, "subject_user required")
		return
	}
	if req.Method == "" {
		req.Method = "admin-recorded"
	}
	if err := a.Store.RecordConsent(r.Context(), req.SubjectUser, req.MachineID, req.Method, req.PolicyVersion); err != nil {
		writeErr(w, http.StatusInternalServerError, "store error")
		return
	}
	_ = a.Store.Audit(r.Context(), currentUser(r).Email, "record_consent", req.SubjectUser,
		map[string]interface{}{"method": req.Method})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type upsertPolicyReq struct {
	Name   string       `json:"name"`
	Policy model.Policy `json:"policy"`
}

type groupPolicyResp struct {
	Group   string       `json:"group"`
	Version int          `json:"version"`
	Policy  model.Policy `json:"policy"`
}

// handleGetPolicy returns the policy currently applied to a group (or the
// built-in default) for the Settings UI to edit. Group defaults to "default".
func (a *App) handleGetPolicy(w http.ResponseWriter, r *http.Request) {
	group := r.URL.Query().Get("group")
	if group == "" {
		group = "default"
	}
	pol, version, err := a.Store.GetGroupPolicy(r.Context(), group)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store error")
		return
	}
	writeJSON(w, http.StatusOK, groupPolicyResp{Group: group, Version: version, Policy: pol})
}

type setPolicyReq struct {
	Group  string       `json:"group"`
	Policy model.Policy `json:"policy"`
}

// handleSetPolicy stores and assigns a group's policy from the Settings UI.
func (a *App) handleSetPolicy(w http.ResponseWriter, r *http.Request) {
	var req setPolicyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad request")
		return
	}
	if req.Group == "" {
		req.Group = "default"
	}
	version, err := a.Store.SetGroupPolicy(r.Context(), req.Group, req.Policy)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store error")
		return
	}
	_ = a.Store.Audit(r.Context(), currentUser(r).Email, "policy_set", req.Group,
		map[string]interface{}{"group": req.Group, "version": version})
	writeJSON(w, http.StatusOK, groupPolicyResp{Group: req.Group, Version: version, Policy: req.Policy})
}

// handleUpsertPolicy stores a new policy version.
func (a *App) handleUpsertPolicy(w http.ResponseWriter, r *http.Request) {
	var req upsertPolicyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeErr(w, http.StatusBadRequest, "name and policy required")
		return
	}
	id, version, err := a.Store.UpsertPolicy(r.Context(), req.Name, req.Policy)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store error")
		return
	}
	_ = a.Store.Audit(r.Context(), currentUser(r).Email, "policy_update", req.Name,
		map[string]interface{}{"version": version})
	writeJSON(w, http.StatusOK, map[string]interface{}{"id": id, "version": version})
}
