// Package blob stores screenshot blobs, encrypted at rest.
//
// Store is the interface the rest of the server depends on. FSStore is a
// filesystem-backed implementation used for development; a MinIO/S3-backed
// implementation satisfying the same interface is the production backend
// (see deploy/docker-compose.yml). Swapping backends requires no changes to
// callers.
package blob

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Store persists opaque, encrypted blobs keyed by an object key.
type Store interface {
	Put(key string, data []byte) error
	Get(key string) ([]byte, error)
	Delete(key string) error
}

// deriveKey turns the configured screenshot key (raw or "base64:..." form)
// into a 32-byte AES-256 key.
func deriveKey(configured string) []byte {
	raw := strings.TrimPrefix(configured, "base64:")
	if b, err := base64.StdEncoding.DecodeString(raw); err == nil && len(b) == 32 {
		return b
	}
	sum := sha256.Sum256([]byte(configured))
	return sum[:]
}

func seal(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func open(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ct, nil)
}

// FSStore is a filesystem-backed encrypted blob store.
type FSStore struct {
	root string
	key  []byte
}

// NewFSStore returns a filesystem store rooted at dir, encrypting with the
// given configured key.
func NewFSStore(dir, configuredKey string) (*FSStore, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &FSStore{root: dir, key: deriveKey(configuredKey)}, nil
}

func (s *FSStore) path(key string) string {
	// Prevent traversal; keys are server-generated but be defensive.
	clean := filepath.Clean("/" + key)
	return filepath.Join(s.root, clean)
}

// Put encrypts and writes data under key.
func (s *FSStore) Put(key string, data []byte) error {
	ct, err := seal(s.key, data)
	if err != nil {
		return err
	}
	p := s.path(key)
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return err
	}
	return os.WriteFile(p, ct, 0o640)
}

// Get reads and decrypts the blob under key.
func (s *FSStore) Get(key string) ([]byte, error) {
	ct, err := os.ReadFile(s.path(key))
	if err != nil {
		return nil, err
	}
	return open(s.key, ct)
}

// Delete removes the blob under key.
func (s *FSStore) Delete(key string) error {
	err := os.Remove(s.path(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Copy is a small helper for streaming callers.
func Copy(dst io.Writer, src io.Reader) (int64, error) { return io.Copy(dst, src) }
