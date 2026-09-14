package api

import (
	"encoding/json"
	"net/http"
)

func (a *App) handleMachineDomains(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	from, to := timeRange(r)
	rows, err := a.Store.Domains(r.Context(), id, from, to, limitParam(r, 200))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query error")
		return
	}
	_ = a.Store.Audit(r.Context(), currentUser(r).Email, "view_domains", id, nil)
	writeJSON(w, http.StatusOK, rows)
}

func (a *App) handleMachineTransfers(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	from, to := timeRange(r)
	rows, err := a.Store.Transfers(r.Context(), id, from, to)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query error")
		return
	}
	_ = a.Store.Audit(r.Context(), currentUser(r).Email, "view_transfers", id, nil)
	writeJSON(w, http.StatusOK, rows)
}

func (a *App) handleMachineDownloads(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	from, to := timeRange(r)
	rows, err := a.Store.Downloads(r.Context(), id, from, to, limitParam(r, 300))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query error")
		return
	}
	_ = a.Store.Audit(r.Context(), currentUser(r).Email, "view_downloads", id, nil)
	writeJSON(w, http.StatusOK, rows)
}

// --- alerts ---

func (a *App) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	unacked := r.URL.Query().Get("unacked") == "true"
	rows, err := a.Store.ListAlerts(r.Context(), unacked, limitParam(r, 200))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query error")
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (a *App) handleAckAlert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.Store.AckAlert(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, "update error")
		return
	}
	_ = a.Store.Audit(r.Context(), currentUser(r).Email, "ack_alert", id, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleListAlertRules(w http.ResponseWriter, r *http.Request) {
	rules, err := a.Store.ListAlertRules(r.Context(), true)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query error")
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

type createRuleReq struct {
	Name   string                 `json:"name"`
	Kind   string                 `json:"kind"`
	Params map[string]interface{} `json:"params"`
}

func (a *App) handleCreateAlertRule(w http.ResponseWriter, r *http.Request) {
	var req createRuleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.Kind == "" {
		writeErr(w, http.StatusBadRequest, "name and kind required")
		return
	}
	switch req.Kind {
	case "domain_blocklist", "large_upload", "app_blocklist",
		"dlp_keyword", "usb_insert", "new_install", "failed_logon":
	default:
		writeErr(w, http.StatusBadRequest, "unknown rule kind")
		return
	}
	id, err := a.Store.CreateAlertRule(r.Context(), req.Name, req.Kind, req.Params)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store error")
		return
	}
	_ = a.Store.Audit(r.Context(), currentUser(r).Email, "create_alert_rule", req.Name,
		map[string]interface{}{"kind": req.Kind})
	writeJSON(w, http.StatusOK, map[string]string{"id": id})
}
