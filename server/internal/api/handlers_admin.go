package api

import (
	"net/http"
	"strconv"
	"time"
)

// timeRange parses ?from and ?to (RFC3339); defaults to the last 24h.
func timeRange(r *http.Request) (time.Time, time.Time) {
	to := time.Now()
	from := to.Add(-24 * time.Hour)
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = t
		}
	}
	return from, to
}

func limitParam(r *http.Request, def int) int {
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			return n
		}
	}
	return def
}

func (a *App) handleListMachines(w http.ResponseWriter, r *http.Request) {
	machines, err := a.Store.ListMachines(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query error")
		return
	}
	writeJSON(w, http.StatusOK, machines)
}

func (a *App) handleMachineScreenshots(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.approvedToView(r, id) {
		writeErr(w, http.StatusForbidden, "approval_required")
		return
	}
	from, to := timeRange(r)
	shots, err := a.Store.ListScreenshots(r.Context(), id, from, to, limitParam(r, 200))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query error")
		return
	}
	// Viewing an individual's screenshot list is a sensitive action -> audit it.
	_ = a.Store.Audit(r.Context(), currentUser(r).Email, "view_screenshots", id,
		map[string]interface{}{"count": len(shots)})
	writeJSON(w, http.StatusOK, shots)
}

func (a *App) handleMachineApps(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	from, to := timeRange(r)
	usage, err := a.Store.AppUsage(r.Context(), id, from, to)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query error")
		return
	}
	_ = a.Store.Audit(r.Context(), currentUser(r).Email, "view_apps", id, nil)
	writeJSON(w, http.StatusOK, usage)
}

func (a *App) handleMachineEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	from, to := timeRange(r)
	kind := r.URL.Query().Get("kind")
	events, err := a.Store.ListEvents(r.Context(), id, kind, from, to, limitParam(r, 500))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query error")
		return
	}
	_ = a.Store.Audit(r.Context(), currentUser(r).Email, "view_events", id,
		map[string]interface{}{"kind": kind})
	writeJSON(w, http.StatusOK, events)
}

func (a *App) handleScreenshotImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	machineID, objectKey, format, err := a.Store.ScreenshotByID(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if !a.approvedToView(r, machineID) {
		writeErr(w, http.StatusForbidden, "approval_required")
		return
	}
	data, err := a.Blob.Get(objectKey)
	if err != nil {
		writeErr(w, http.StatusNotFound, "blob missing")
		return
	}
	_ = a.Store.Audit(r.Context(), currentUser(r).Email, "view_screenshot_image", machineID,
		map[string]interface{}{"screenshot_id": id})
	ct := "image/webp"
	if format == "jpeg" || format == "jpg" {
		ct = "image/jpeg"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(data)
}

// approvedToView reports whether the current user may view screenshots for the
// machine. When dual approval is disabled, everyone with a valid MFA session may;
// when enabled, an active second-admin approval is required.
func (a *App) approvedToView(r *http.Request, machineID string) bool {
	if !a.Cfg.RequireDualApproval {
		return true
	}
	ok, err := a.Store.HasActiveApproval(r.Context(), currentUser(r).Email, machineID)
	return err == nil && ok
}

func (a *App) handleAudit(w http.ResponseWriter, r *http.Request) {
	entries, err := a.Store.ListAudit(r.Context(), limitParam(r, 500))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query error")
		return
	}
	writeJSON(w, http.StatusOK, entries)
}
