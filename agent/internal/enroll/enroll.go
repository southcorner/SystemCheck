// Package enroll performs one-time agent enrollment: generate a key + CSR,
// exchange the enrollment token for a signed client certificate, and persist it.
package enroll

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"os"
	"runtime"

	"github.com/southcorner/systemcheck/agent/internal/config"
	"github.com/southcorner/systemcheck/agent/internal/transport"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// Run enrolls the agent if it is not already enrolled. It returns the machine id.
func Run(ctx context.Context, cfg *config.Config, agentVersion string) (string, error) {
	if cfg.Enrolled() {
		return cfg.MachineID, nil
	}
	if cfg.EnrollToken == "" {
		return "", fmt.Errorf("not enrolled and no enroll token provided (set SC_ENROLL_TOKEN)")
	}

	// Generate a private key and CSR.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", err
	}
	hostname, _ := os.Hostname()
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: hostname},
	}, key)
	if err != nil {
		return "", err
	}
	csrPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER}))

	client := transport.NewEnrollClient(cfg.ServerURL, nil)
	resp, err := client.Enroll(ctx, wire.EnrollRequest{
		Token:        cfg.EnrollToken,
		Hostname:     hostname,
		OS:           runtime.GOOS,
		OSVersion:    osVersion(),
		AgentVersion: agentVersion,
		CSRPEM:       csrPEM,
	})
	if err != nil {
		return "", err
	}

	// Persist cert, CA, and key.
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(cfg.KeyPath(), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		return "", err
	}
	if err := os.WriteFile(cfg.CertPath(), []byte(resp.CertPEM), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(cfg.CAPath(), []byte(resp.CAPEM), 0o644); err != nil {
		return "", err
	}
	cfg.MachineID = resp.MachineID
	cfg.EnrollToken = "" // consumed
	if err := cfg.SaveState(); err != nil {
		return "", err
	}
	return resp.MachineID, nil
}
