package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// EnsureDevCerts generates a self-signed CA and a server certificate for local
// development if the files do not already exist. It is a convenience for `go run`
// and MUST NOT be used to provision production certificates.
//
// extraSANs adds Subject Alternative Names beyond localhost so agents on other
// machines can reach the server by its LAN IP or hostname (each entry is treated
// as an IP if it parses as one, otherwise a DNS name).
func EnsureDevCerts(caCert, caKey, srvCert, srvKey string, extraSANs ...string) error {
	if fileExists(caCert) && fileExists(caKey) && fileExists(srvCert) && fileExists(srvKey) {
		return nil
	}
	for _, p := range []string{caCert, caKey, srvCert, srvKey} {
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			return err
		}
	}

	// CA
	caKeyPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "SystemCheck Dev CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKeyPriv.PublicKey, caKeyPriv)
	if err != nil {
		return err
	}
	if err := writeCert(caCert, caDER); err != nil {
		return err
	}
	if err := writeKey(caKey, caKeyPriv); err != nil {
		return err
	}

	// Server cert signed by the CA.
	caParsed, err := x509.ParseCertificate(caDER)
	if err != nil {
		return err
	}
	srvKeyPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	dnsNames := []string{"localhost"}
	ips := []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback}
	for _, s := range extraSANs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if ip := net.ParseIP(s); ip != nil {
			ips = append(ips, ip)
		} else {
			dnsNames = append(dnsNames, s)
		}
	}
	srvTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "systemcheck"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(2 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     dnsNames,
		IPAddresses:  ips,
	}
	srvDER, err := x509.CreateCertificate(rand.Reader, srvTmpl, caParsed, &srvKeyPriv.PublicKey, caKeyPriv)
	if err != nil {
		return err
	}
	if err := writeCert(srvCert, srvDER); err != nil {
		return err
	}
	return writeKey(srvKey, srvKeyPriv)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func writeCert(path string, der []byte) error {
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644)
}

func writeKey(path string, key *ecdsa.PrivateKey) error {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600)
}
