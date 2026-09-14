// Package release provides ed25519 signing and verification for agent binaries.
// The private key is held offline; the server advertises the public key's
// signature via GET /v1/agent-version, and agents verify downloads against a
// pinned copy of the public key before installing.
package release

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
)

// GenerateKey returns a new ed25519 keypair as base64 strings (public, private).
func GenerateKey() (pubB64, privB64 string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(pub),
		base64.StdEncoding.EncodeToString(priv), nil
}

// Sign signs data with a base64-encoded ed25519 private key, returning a base64
// signature.
func Sign(privB64 string, data []byte) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(privB64)
	if err != nil {
		return "", err
	}
	if len(raw) != ed25519.PrivateKeySize {
		return "", errors.New("invalid private key size")
	}
	sig := ed25519.Sign(ed25519.PrivateKey(raw), data)
	return base64.StdEncoding.EncodeToString(sig), nil
}

// Verify checks a base64 signature over data against a base64 public key.
func Verify(pubB64, sigB64 string, data []byte) bool {
	pub, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return false
	}
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(pub), data, sig)
}
