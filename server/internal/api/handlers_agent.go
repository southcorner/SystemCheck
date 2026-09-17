package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/southcorner/systemcheck/server/internal/alert"
	"github.com/southcorner/systemcheck/server/internal/auth"
	"github.com/southcorner/systemcheck/server/internal/model"
	"github.com/southcorner/systemcheck/server/internal/notify"
)

const clientCertTTL = 365 * 24 * time.Hour

// handleEnroll exchanges a one-time token for a signed client certificate.
func (a *App) handleEnroll(w http.ResponseWriter, r *http.Request) {
	var req model.EnrollRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad request")
		return
	}
	if req.Token == "" || req.CSRPEM == "" || req.Hostname == "" {
		writeErr(w, http.StatusBadRequest, "token, hostname and csr_pem required")
		return
	}
	tok, err := a.Store.ConsumeEnrollmentToken(r.Context(), auth.HashToken(req.Token))
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid or expired enrollment token")
		return
	}
	os := req.OS
	if os == "" {
		os = "windows"
	}
	// Sign first with a placeholder CN, then use the machine id as CN by
	// creating the machine row first.
	machineID := ""
	// Create the machine row (fingerprint filled after signing).
	// We sign, get fingerprint, then insert. Insert needs a CN; use hostname.
	certPEM, fp, err := a.CA.SignCSR(req.CSRPEM, req.Hostname, clientCertTTL)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "csr: "+err.Error())
		return
	}
	machineID, err = a.Store.CreateMachine(r.Context(),
		req.Hostname, os, req.OSVersion, req.AgentVersion, tok.Group, tok.AssignedUser, fp)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "enroll store error")
		return
	}
	_ = a.Store.Audit(r.Context(), "agent:"+req.Hostname, "agent_enrolled", machineID,
		map[string]interface{}{"group": tok.Group, "user": tok.AssignedUser})
	writeJSON(w, http.StatusOK, model.EnrollResponse{
		MachineID: machineID,
		CertPEM:   certPEM,
		CAPEM:     string(a.CA.CAPEM()),
	})
}

// handlePolicy returns the effective policy for the calling agent.
func (a *App) handlePolicy(w http.ResponseWriter, r *http.Request) {
	m := currentMachine(r)
	pol, err := a.Store.EffectivePolicy(r.Context(), m)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "policy error")
		return
	}
	writeJSON(w, http.StatusOK, pol)
}

// handleIngest accepts a batch of events.
func (a *App) handleIngest(w http.ResponseWriter, r *http.Request) {
	m := currentMachine(r)
	var batch model.IngestBatch
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(&batch); err != nil {
		writeErr(w, http.StatusBadRequest, "bad batch")
		return
	}
	n, err := a.Store.InsertEvents(r.Context(), m.ID, batch.Events)
	if err != nil {
		a.Log.Printf("ingest error machine=%s: %v", m.ID, err)
		writeErr(w, http.StatusInternalServerError, "ingest error")
		return
	}
	// Evaluate alert rules against the batch (best-effort; never fails ingest).
	notifyFn := func(mt alert.Match) {
		_ = a.Notifier.Send(r.Context(), notify.AlertPayload{
			MachineID: m.ID, Severity: mt.Severity, Message: mt.Message, Data: mt.Data,
		})
	}
	if err := alert.Process(r.Context(), a.Store, m.ID, batch.Events, notifyFn); err != nil {
		a.Log.Printf("alert eval error machine=%s: %v", m.ID, err)
	}
	writeJSON(w, http.StatusOK, model.IngestResponse{Accepted: n, Rejected: len(batch.Events) - n})
}

// handleScreenshotUpload stores an encrypted screenshot blob.
func (a *App) handleScreenshotUpload(w http.ResponseWriter, r *http.Request) {
	m := currentMachine(r)
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "bad upload")
		return
	}
	key := r.FormValue("object_key")
	if key == "" {
		writeErr(w, http.StatusBadRequest, "object_key required")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "file required")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 16<<20))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "read error")
		return
	}
	// Namespace blobs by machine to keep keys unique.
	fullKey := fmt.Sprintf("%s/%s", m.ID, key)
	if err := a.Blob.Put(fullKey, data); err != nil {
		writeErr(w, http.StatusInternalServerError, "store error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"object_key": fullKey})
}

// handleHeartbeat updates machine liveness.
func (a *App) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	m := currentMachine(r)
	var hb model.Heartbeat
	_ = json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&hb)
	_ = a.Store.TouchMachine(r.Context(), m.ID, hb.AgentVersion)

	// Attribute the machine to the real logged-in user reported by the session
	// helper, so per-person data and the consent gate track the actual employee.
	if hb.InteractiveUser != "" && hb.InteractiveUser != m.AssignedUser {
		if err := a.Store.SetMachineAssignedUser(r.Context(), m.ID, hb.InteractiveUser); err == nil {
			m.AssignedUser = hb.InteractiveUser
		}
	}
	// Record the user's in-session acceptance of the monitoring notice. This is
	// the consent gate EffectivePolicy checks before activating collection.
	if hb.Consented && hb.InteractiveUser != "" {
		if ok, _ := a.Store.HasConsent(r.Context(), hb.InteractiveUser); !ok {
			_ = a.Store.RecordConsent(r.Context(), hb.InteractiveUser, m.ID, "agent-clickthrough", hb.PolicyVersion)
		}
	}
	// Collector health: store the latest report and alert on any collector that
	// newly transitioned to DOWN (compared to the previous report), so a stopped
	// collector (or a killed session helper) surfaces without spamming.
	if len(hb.Collectors) > 0 {
		prev, _, _ := a.Store.GetMachineHealth(r.Context(), m.ID)
		prevDown := map[string]bool{}
		for _, c := range prev {
			if !c.Running {
				prevDown[c.Name] = true
			}
		}
		_ = a.Store.SetMachineHealth(r.Context(), m.ID, hb.Collectors)
		for _, c := range hb.Collectors {
			if !c.Running && !prevDown[c.Name] {
				msg := fmt.Sprintf("Collector %q stopped on %s", c.Name, m.Hostname)
				_ = a.Store.CreateAlert(r.Context(), "agent_health", m.ID, "warning", msg,
					map[string]interface{}{"collector": c.Name, "role": c.Role, "error": c.Error})
			}
		}
	}

	// Advertise the latest release so the agent can update promptly on its next
	// heartbeat rather than waiting for the periodic check.
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "update_version": a.Cfg.AgentLatestVersion})
}
