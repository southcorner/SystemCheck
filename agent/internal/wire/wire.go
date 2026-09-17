// Package wire defines the agent side of the HTTPS/JSON contract with the
// server (see proto/agent.md). It intentionally mirrors the server's model
// package; the two are separate Go modules.
package wire

import "time"

// EnrollRequest is sent to POST /v1/enroll.
type EnrollRequest struct {
	Token        string `json:"token"`
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	OSVersion    string `json:"os_version"`
	AgentVersion string `json:"agent_version"`
	CSRPEM       string `json:"csr_pem"`
}

// EnrollResponse is returned from POST /v1/enroll.
type EnrollResponse struct {
	MachineID string `json:"machine_id"`
	CertPEM   string `json:"cert_pem"`
	CAPEM     string `json:"ca_pem"`
}

// Policy is returned from GET /v1/policy.
type Policy struct {
	Version      int              `json:"version"`
	Active       bool             `json:"active"`
	Screenshot   ScreenshotPolicy `json:"screenshot"`
	Foreground   ForegroundPolicy `json:"foreground"`
	DNS          Toggle           `json:"dns"`
	Netflow      NetflowPolicy    `json:"netflow"`
	Fswatch      FswatchPolicy    `json:"fswatch"`
	Browsing     Toggle           `json:"browsing"`
	USB          Toggle           `json:"usb"`
	PrintJobs    Toggle           `json:"printjobs"`
	Installs     Toggle           `json:"installs"`
	Posture      PosturePolicy    `json:"posture"`
	Seclog       Toggle           `json:"seclog"`
	Exclusions   Exclusions       `json:"exclusions"`
	HeartbeatSec int              `json:"heartbeat_sec"`
}

// ScreenshotPolicy configures screenshot capture.
type ScreenshotPolicy struct {
	Enabled        bool   `json:"enabled"`
	MinIntervalSec int    `json:"min_interval_sec"`
	MaxIntervalSec int    `json:"max_interval_sec"`
	MaxWidth       int    `json:"max_width"`
	Format         string `json:"format"`
	Quality        int    `json:"quality"`
}

// ForegroundPolicy configures foreground-app tracking.
type ForegroundPolicy struct {
	Enabled          bool `json:"enabled"`
	PollSec          int  `json:"poll_sec"`
	IdleThresholdSec int  `json:"idle_threshold_sec"`
}

// Toggle is a simple on/off switch.
type Toggle struct {
	Enabled bool `json:"enabled"`
}

// NetflowPolicy configures per-process network byte accounting.
type NetflowPolicy struct {
	Enabled   bool `json:"enabled"`
	RollupSec int  `json:"rollup_sec"` // aggregation window; default 60
}

// FswatchPolicy configures folder watching.
type FswatchPolicy struct {
	Enabled               bool     `json:"enabled"`
	Folders               []string `json:"folders"`
	IncludeBrowserHistory bool     `json:"include_browser_history"`
}

// PosturePolicy configures periodic device-posture snapshots.
type PosturePolicy struct {
	Enabled     bool `json:"enabled"`
	IntervalSec int  `json:"interval_sec"` // default 3600
}

// Exclusions lists domains/processes never recorded.
type Exclusions struct {
	Domains   []string `json:"domains"`
	Processes []string `json:"processes"`
}

// Event is a single observation emitted by a collector.
type Event struct {
	Kind string                 `json:"kind"`
	TS   time.Time              `json:"ts"`
	Data map[string]interface{} `json:"data"`
}

// IngestBatch is posted to POST /v1/ingest.
type IngestBatch struct {
	BatchID string    `json:"batch_id"`
	SentAt  time.Time `json:"sent_at"`
	Events  []Event   `json:"events"`
}

// Heartbeat is posted to POST /v1/heartbeat.
type Heartbeat struct {
	AgentVersion  string `json:"agent_version"`
	PolicyVersion int    `json:"policy_version"`
	QueuedEvents  int    `json:"queued_events"`
	Healthy       bool   `json:"healthy"`
	// InteractiveUser and Consented carry the logged-in identity and consent
	// state observed by the user-session helper (via the session handoff file).
	InteractiveUser string `json:"interactive_user,omitempty"`
	Consented       bool   `json:"consented,omitempty"`
	// Collectors is the health of each collector (service + session), so the
	// server can flag/alert when something that should be running isn't.
	Collectors []CollectorStatus `json:"collectors,omitempty"`
}

// CollectorStatus reports whether one collector is running, with its last error.
type CollectorStatus struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
	Error   string `json:"error,omitempty"`
	Role    string `json:"role"` // "service" or "session"
}

// HeartbeatResponse is the server's reply to a heartbeat. UpdateVersion lets
// the server nudge the agent to update promptly (on its next ~60s heartbeat)
// instead of waiting for the periodic update check. Command lets the server ask
// the agent to do something on its next heartbeat ("restart", "sendlog").
type HeartbeatResponse struct {
	OK            bool   `json:"ok"`
	UpdateVersion string `json:"update_version"`
	Command       string `json:"command,omitempty"`
}

// SessionState is the handoff file the user-session helper writes into the
// spool dir and the service reads: who is logged in and whether they accepted
// the monitoring notice. Kept in the (user-writable) spool so the unprivileged
// helper needs no access to the service's private data dir / certificates.
type SessionState struct {
	InteractiveUser string            `json:"interactive_user"`
	Consented       bool              `json:"consented"`
	UpdatedAt       string            `json:"updated_at"`
	Collectors      []CollectorStatus `json:"collectors,omitempty"` // session-helper collector health
}

// Runtime is the handoff file the service writes into the spool dir for the
// helper: the machine id (so blob object keys are namespaced identically) and
// the current effective policy (so the helper needs no server credentials).
type Runtime struct {
	MachineID string `json:"machine_id"`
	Policy    Policy `json:"policy"`
}
