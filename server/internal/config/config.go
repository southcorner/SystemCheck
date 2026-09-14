// Package config loads server configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all server configuration.
type Config struct {
	ListenAddr string
	TLSCert    string
	TLSKey     string
	CACert     string
	CAKey      string

	DBURL string

	S3Endpoint string
	S3Bucket   string
	S3UseSSL   bool
	S3Access   string
	S3Secret   string

	ScreenshotKey string
	SessionSecret string
	EnrollSecret  string

	BootstrapAdminEmail    string
	BootstrapAdminPassword string

	// Retention windows in days per data class (0 disables purge for that class).
	RetentionScreenshotsDays int
	RetentionActivityDays    int // app usage, domains, transfers, downloads
	RetentionSecurityDays    int // usb, printjob, install, posture, seclog
	RetentionAuditDays       int

	// RequireDualApproval gates viewing an individual's screenshots on a second
	// admin's approval.
	RequireDualApproval bool

	// Agent release advertisement (see GET /v1/agent-version).
	AgentLatestVersion string
	AgentDownloadURL   string
	AgentSignatureB64  string // ed25519 signature of the release, base64

	// AlertWebhook, if set, receives alert JSON (SIEM/webhook export).
	AlertWebhook string

	// TLSSANs are extra Subject Alternative Names (LAN IPs/hostnames) added to the
	// auto-generated dev server certificate so agents on other machines can reach
	// it. Comma-separated in SC_TLS_SANS. Ignored once real certs are provided.
	TLSSANs []string
}

// Load reads configuration from environment variables, applying defaults.
func Load() (*Config, error) {
	c := &Config{
		ListenAddr:             env("SC_LISTEN_ADDR", ":8443"),
		TLSCert:                env("SC_TLS_CERT", "./certs/server.crt"),
		TLSKey:                 env("SC_TLS_KEY", "./certs/server.key"),
		CACert:                 env("SC_CA_CERT", "./certs/ca.crt"),
		CAKey:                  env("SC_CA_KEY", "./certs/ca.key"),
		S3Endpoint:             env("SC_S3_ENDPOINT", "127.0.0.1:9000"),
		S3Bucket:               env("SC_S3_BUCKET", "screenshots"),
		S3UseSSL:               env("SC_S3_USE_SSL", "false") == "true",
		S3Access:               env("MINIO_ROOT_USER", ""),
		S3Secret:               env("MINIO_ROOT_PASSWORD", ""),
		ScreenshotKey:          env("SC_SCREENSHOT_KEY", ""),
		SessionSecret:          env("SC_SESSION_SECRET", ""),
		EnrollSecret:           env("SC_ENROLL_SECRET", ""),
		BootstrapAdminEmail:    env("SC_BOOTSTRAP_ADMIN_EMAIL", ""),
		BootstrapAdminPassword: env("SC_BOOTSTRAP_ADMIN_PASSWORD", ""),

		RetentionScreenshotsDays: envInt("SC_RETENTION_SCREENSHOTS_DAYS", 30),
		RetentionActivityDays:    envInt("SC_RETENTION_ACTIVITY_DAYS", 90),
		RetentionSecurityDays:    envInt("SC_RETENTION_SECURITY_DAYS", 180),
		RetentionAuditDays:       envInt("SC_RETENTION_AUDIT_DAYS", 365),

		RequireDualApproval: env("SC_REQUIRE_DUAL_APPROVAL", "false") == "true",

		AgentLatestVersion: env("SC_AGENT_LATEST_VERSION", ""),
		AgentDownloadURL:   env("SC_AGENT_DOWNLOAD_URL", ""),
		AgentSignatureB64:  env("SC_AGENT_SIGNATURE_B64", ""),

		AlertWebhook: env("SC_ALERT_WEBHOOK", ""),
		TLSSANs:      splitCSV(env("SC_TLS_SANS", "")),
	}

	host := env("DB_HOST", "127.0.0.1")
	port := env("DB_PORT", "5432")
	user := env("DB_USER", "systemcheck")
	pass := env("DB_PASSWORD", "")
	name := env("DB_NAME", "systemcheck")
	c.DBURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, name)

	if c.SessionSecret == "" {
		return nil, fmt.Errorf("SC_SESSION_SECRET is required")
	}
	return c, nil
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
