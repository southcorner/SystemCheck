// Package model holds the shared data types for the SystemCheck server,
// including the agent<->server wire types documented in proto/agent.md.
package model

import "time"

// Role is an admin RBAC role.
type Role string

const (
	RoleAdmin   Role = "admin"
	RoleAuditor Role = "auditor"
	RoleViewer  Role = "viewer"
)

// ValidRole reports whether r is a known role.
func ValidRole(r Role) bool {
	return r == RoleAdmin || r == RoleAuditor || r == RoleViewer
}

// EnrollRequest is the body of POST /v1/enroll.
type EnrollRequest struct {
	Token        string `json:"token"`
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	OSVersion    string `json:"os_version"`
	AgentVersion string `json:"agent_version"`
	CSRPEM       string `json:"csr_pem"`
}

// EnrollResponse is the response to a successful enrollment.
type EnrollResponse struct {
	MachineID string `json:"machine_id"`
	CertPEM   string `json:"cert_pem"`
	CAPEM     string `json:"ca_pem"`
}

// Policy is the effective policy returned by GET /v1/policy.
type Policy struct {
	Version      int              `json:"version"`
	Active       bool             `json:"active"`
	Screenshot   ScreenshotPolicy `json:"screenshot"`
	Foreground   ForegroundPolicy `json:"foreground"`
	DNS          TogglePolicy     `json:"dns"`
	Netflow      NetflowPolicy    `json:"netflow"`
	Fswatch      FswatchPolicy    `json:"fswatch"`
	Browsing     TogglePolicy     `json:"browsing"`
	USB          TogglePolicy     `json:"usb"`
	PrintJobs    TogglePolicy     `json:"printjobs"`
	Installs     TogglePolicy     `json:"installs"`
	Posture      PosturePolicy    `json:"posture"`
	Seclog       TogglePolicy     `json:"seclog"`
	Exclusions   Exclusions       `json:"exclusions"`
	HeartbeatSec int              `json:"heartbeat_sec"`
}

// ScreenshotPolicy configures the screenshot collector.
type ScreenshotPolicy struct {
	Enabled        bool         `json:"enabled"`
	MinIntervalSec int          `json:"min_interval_sec"`
	MaxIntervalSec int          `json:"max_interval_sec"`
	MaxWidth       int          `json:"max_width"`
	Format         string       `json:"format"`  // webp|jpeg
	Quality        int          `json:"quality"` // 1-100
	BlurRegions    []BlurRegion `json:"blur_regions"`
}

// BlurRegion is a rectangle blurred out of every screenshot for privacy.
type BlurRegion struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// ForegroundPolicy configures the foreground-app collector.
type ForegroundPolicy struct {
	Enabled          bool `json:"enabled"`
	PollSec          int  `json:"poll_sec"`
	IdleThresholdSec int  `json:"idle_threshold_sec"`
}

// TogglePolicy is a simple on/off collector switch.
type TogglePolicy struct {
	Enabled bool `json:"enabled"`
}

// NetflowPolicy configures per-process network byte accounting.
type NetflowPolicy struct {
	Enabled   bool `json:"enabled"`
	RollupSec int  `json:"rollup_sec"`
}

// FswatchPolicy configures the filesystem-watch collector.
type FswatchPolicy struct {
	Enabled               bool     `json:"enabled"`
	Folders               []string `json:"folders"`
	IncludeBrowserHistory bool     `json:"include_browser_history"`
}

// PosturePolicy configures periodic device-posture snapshots.
type PosturePolicy struct {
	Enabled     bool `json:"enabled"`
	IntervalSec int  `json:"interval_sec"`
}

// Exclusions lists domains/processes never recorded (privacy).
type Exclusions struct {
	Domains   []string `json:"domains"`
	Processes []string `json:"processes"`
}

// Event is a single collected observation.
type Event struct {
	Kind string                 `json:"kind"`
	TS   time.Time              `json:"ts"`
	Data map[string]interface{} `json:"data"`
}

// IngestBatch is the body of POST /v1/ingest.
type IngestBatch struct {
	BatchID string    `json:"batch_id"`
	SentAt  time.Time `json:"sent_at"`
	Events  []Event   `json:"events"`
}

// IngestResponse reports how many events were accepted.
type IngestResponse struct {
	Accepted int `json:"accepted"`
	Rejected int `json:"rejected"`
}

// Heartbeat is the body of POST /v1/heartbeat.
type Heartbeat struct {
	AgentVersion  string `json:"agent_version"`
	PolicyVersion int    `json:"policy_version"`
	QueuedEvents  int    `json:"queued_events"`
	Healthy       bool   `json:"healthy"`
	// InteractiveUser is the logged-in Windows user the session helper observed
	// (e.g. "DOMAIN\\alice"); the server attributes the machine to this person.
	InteractiveUser string `json:"interactive_user,omitempty"`
	// Consented reports that the user accepted the monitoring notice in-session.
	Consented bool `json:"consented,omitempty"`
}

// DefaultPolicy returns a conservative, privacy-first default policy that is
// inactive until consent is recorded.
func DefaultPolicy() Policy {
	return Policy{
		Version: 1,
		Active:  false,
		Screenshot: ScreenshotPolicy{
			Enabled: true, MinIntervalSec: 180, MaxIntervalSec: 900,
			MaxWidth: 1600, Format: "webp", Quality: 60,
		},
		Foreground:   ForegroundPolicy{Enabled: true, PollSec: 5, IdleThresholdSec: 120},
		DNS:          TogglePolicy{Enabled: true},
		Netflow:      NetflowPolicy{Enabled: true, RollupSec: 60},
		Fswatch:      FswatchPolicy{Enabled: true, Folders: []string{`%USERPROFILE%\Downloads`}},
		Browsing:     TogglePolicy{Enabled: true},
		USB:          TogglePolicy{Enabled: true},
		PrintJobs:    TogglePolicy{Enabled: true},
		Installs:     TogglePolicy{Enabled: true},
		Posture:      PosturePolicy{Enabled: true, IntervalSec: 3600},
		Seclog:       TogglePolicy{Enabled: true},
		HeartbeatSec: 60,
	}
}
