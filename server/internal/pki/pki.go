// Package pki loads the enrollment CA and signs agent client certificates.
package pki

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"time"
)

// CA signs agent client certificates.
type CA struct {
	cert  *x509.Certificate
	key   any
	caPEM []byte
}

// LoadCA reads a CA certificate and private key from PEM files.
func LoadCA(certPath, keyPath string) (*CA, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("read ca cert: %w", err)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read ca key: %w", err)
	}
	cb, _ := pem.Decode(certPEM)
	if cb == nil {
		return nil, errors.New("invalid ca cert PEM")
	}
	cert, err := x509.ParseCertificate(cb.Bytes)
	if err != nil {
		return nil, err
	}
	kb, _ := pem.Decode(keyPEM)
	if kb == nil {
		return nil, errors.New("invalid ca key PEM")
	}
	key, err := parseKey(kb.Bytes)
	if err != nil {
		return nil, err
	}
	return &CA{cert: cert, key: key, caPEM: certPEM}, nil
}

func parseKey(der []byte) (any, error) {
	if k, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		return k, nil
	}
	if k, err := x509.ParseECPrivateKey(der); err == nil {
		return k, nil
	}
	if k, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return k, nil
	}
	return nil, errors.New("unsupported ca private key format")
}

// CAPEM returns the CA certificate in PEM form (handed to agents).
func (c *CA) CAPEM() []byte { return c.caPEM }

// SignCSR signs a PEM-encoded certificate request as a client certificate whose
// common name identifies the machine. It returns the certificate PEM and its
// SHA-256 fingerprint (hex).
func (c *CA) SignCSR(csrPEM string, commonName string, ttl time.Duration) (certPEM string, fingerprint string, err error) {
	block, _ := pem.Decode([]byte(csrPEM))
	if block == nil {
		return "", "", errors.New("invalid CSR PEM")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return "", "", err
	}
	if err := csr.CheckSignature(); err != nil {
		return "", "", fmt.Errorf("csr signature: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      csr.Subject,
		NotBefore:    time.Now().Add(-5 * time.Minute),
		NotAfter:     time.Now().Add(ttl),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	tmpl.Subject.CommonName = commonName
	der, err := x509.CreateCertificate(rand.Reader, tmpl, c.cert, csr.PublicKey, c.key)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256(der)
	fingerprint = hex.EncodeToString(sum[:])
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	return certPEM, fingerprint, nil
}

// Fingerprint returns the SHA-256 hex of a parsed certificate's raw DER.
func Fingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])
}
