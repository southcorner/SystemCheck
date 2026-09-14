package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"path/filepath"
	"testing"
	"time"
)

func TestEnsureDevCertsAndSignCSR(t *testing.T) {
	dir := t.TempDir()
	caCert := filepath.Join(dir, "ca.crt")
	caKey := filepath.Join(dir, "ca.key")
	srvCert := filepath.Join(dir, "server.crt")
	srvKey := filepath.Join(dir, "server.key")
	if err := EnsureDevCerts(caCert, caKey, srvCert, srvKey); err != nil {
		t.Fatalf("ensure dev certs: %v", err)
	}
	ca, err := LoadCA(caCert, caKey)
	if err != nil {
		t.Fatalf("load ca: %v", err)
	}

	// Build a client CSR and have the CA sign it.
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csrDER, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "TEST-PC"},
	}, key)
	csrPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER}))

	certPEM, fp, err := ca.SignCSR(csrPEM, "TEST-PC", time.Hour)
	if err != nil {
		t.Fatalf("sign csr: %v", err)
	}
	if fp == "" || len(certPEM) == 0 {
		t.Fatal("empty cert or fingerprint")
	}
	// The signed cert must verify against the CA.
	block, _ := pem.Decode([]byte(certPEM))
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse signed cert: %v", err)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(ca.CAPEM())
	if _, err := cert.Verify(x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		t.Fatalf("verify signed cert against CA: %v", err)
	}
	if Fingerprint(cert) != fp {
		t.Fatal("fingerprint mismatch")
	}
}
