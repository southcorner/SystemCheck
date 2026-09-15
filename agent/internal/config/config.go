// Package config loads agent configuration from a JSON file and environment.
package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
)

// DefaultDataDir returns the platform default data directory (used for early
// logging before the config file is read).
func DefaultDataDir() string { return defaultDataDir() }

// Config is the agent's on-disk configuration.
type Config struct {
	ServerURL   string `json:"server_url"`   // e.g. https://systemcheck.corp:8443
	DataDir     string `json:"data_dir"`     // spool + certs live here
	EnrollToken string `json:"enroll_token"` // one-time, cleared after enrollment
	// Populated after enrollment:
	MachineID string `json:"machine_id"`
}

// Paths derived from DataDir.
func (c *Config) SpoolDir() string  { return filepath.Join(c.DataDir, "spool") }
func (c *Config) CertPath() string  { return filepath.Join(c.DataDir, "agent.crt") }
func (c *Config) KeyPath() string   { return filepath.Join(c.DataDir, "agent.key") }
func (c *Config) CAPath() string    { return filepath.Join(c.DataDir, "ca.crt") }
func (c *Config) statePath() string { return filepath.Join(c.DataDir, "state.json") }

// Load reads config from path, applying env overrides and defaults.
func Load(path string) (*Config, error) {
	c := &Config{DataDir: defaultDataDir()}
	if b, err := os.ReadFile(path); err == nil {
		// Tolerate a UTF-8 BOM: Windows PowerShell's Set-Content -Encoding UTF8
		// prepends one, which the JSON parser would otherwise reject.
		b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
		if err := json.Unmarshal(b, c); err != nil {
			return nil, err
		}
	}
	if v := os.Getenv("SC_SERVER_URL"); v != "" {
		c.ServerURL = v
	}
	if v := os.Getenv("SC_DATA_DIR"); v != "" {
		c.DataDir = v
	}
	if v := os.Getenv("SC_ENROLL_TOKEN"); v != "" {
		c.EnrollToken = v
	}
	if c.DataDir == "" {
		c.DataDir = "."
	}
	_ = os.MkdirAll(c.SpoolDir(), 0o750)
	// Merge previously saved state (machine id).
	if b, err := os.ReadFile(c.statePath()); err == nil {
		var st Config
		if json.Unmarshal(b, &st) == nil && st.MachineID != "" {
			c.MachineID = st.MachineID
		}
	}
	return c, nil
}

// SaveState persists post-enrollment state (machine id).
func (c *Config) SaveState() error {
	b, err := json.MarshalIndent(struct {
		MachineID string `json:"machine_id"`
	}{c.MachineID}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.statePath(), b, 0o600)
}

// Enrolled reports whether a client certificate already exists.
func (c *Config) Enrolled() bool {
	_, err := os.Stat(c.CertPath())
	return err == nil
}
