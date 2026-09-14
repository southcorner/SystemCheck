// Package config loads server configuration from the environment.
package config

import (
	"fmt"
	"os"
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
