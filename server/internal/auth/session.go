package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// NewToken returns a cryptographically random opaque token (hex).
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// HashToken returns the SHA-256 hex of a token, for storage/lookup so that raw
// tokens are never persisted.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
