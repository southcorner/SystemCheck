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
}
