// Package updater performs signed agent self-updates. The agent advertises its
// version to the server; the server advertises the latest signed release; the
// agent downloads it, verifies an ed25519 signature against a pinned public key,
// and installs it (platform-specific swap + service restart).
//
// Updates are DISABLED unless PinnedPublicKey is set at build time:
//
//	go build -ldflags "-X github.com/southcorner/systemcheck/agent/internal/updater.PinnedPublicKey=<base64>"
package updater

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Install-attempt back-off. If a release is downloaded and installed but the
// agent keeps coming back on the OLD version (a swap that silently fails, e.g.
// the binary could not be replaced), then without a guard every agent start
// re-downloads and re-restarts - a fleet-wide download/restart storm. So each
// target version gets a bounded number of attempts before we back off and wait.
const (
	maxInstallAttempts = 3
	attemptCooldown    = 6 * time.Hour
)

// attemptRecord persists how many times we have tried to install a version.
type attemptRecord struct {
	Version string    `json:"version"`
	Count   int       `json:"count"`
	At      time.Time `json:"at"`
}

// attemptPath is the state file, kept next to the agent binary.
func attemptPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "update-attempt.json")
}

func loadAttempt() attemptRecord {
	var rec attemptRecord
	p := attemptPath()
	if p == "" {
		return rec
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return rec
	}
	_ = json.Unmarshal(data, &rec)
	return rec
}

// shouldAttempt reports whether we may try to install target now, and returns
// the record to carry the attempt count forward.
func shouldAttempt(target string) (attemptRecord, bool) {
	rec := loadAttempt()
	if rec.Version != target {
		return attemptRecord{Version: target}, true // new target: fresh budget
	}
	if rec.Count < maxInstallAttempts {
		return rec, true
	}
	if time.Since(rec.At) > attemptCooldown {
		return attemptRecord{Version: target}, true // cooled down: try again
	}
	return rec, false
}

func saveAttempt(rec attemptRecord) {
	p := attemptPath()
	if p == "" {
		return
	}
	rec.At = time.Now()
	rec.Count++
	if data, err := json.Marshal(rec); err == nil {
		_ = os.WriteFile(p, data, 0o600)
	}
}

// PinnedPublicKey is the base64 ed25519 public key releases are signed with.
// Empty (the default) disables self-update entirely.
var PinnedPublicKey = ""

// VersionInfo describes an advertised release.
type VersionInfo struct {
	Version   string
	URL       string
	Signature string
}

// GetFunc downloads the bytes at a URL.
type GetFunc func(ctx context.Context, url string) ([]byte, error)

// InstallFunc replaces the running binary with the new bytes and arranges a
// restart. It is platform-specific (see install_windows.go / install_other.go).
type InstallFunc func(data []byte) error

// Enabled reports whether self-update is configured.
func Enabled() bool { return PinnedPublicKey != "" }

// CheckAndUpdate downloads, verifies, and installs a newer signed release.
// Returns (updated, error). It is a no-op (false, nil) when disabled, when the
// advertised version is empty, or when it is not newer than current.
func CheckAndUpdate(ctx context.Context, current string, info VersionInfo, get GetFunc, install InstallFunc) (bool, error) {
	if !Enabled() || info.Version == "" || info.URL == "" {
		return false, nil
	}
	if !isNewer(info.Version, current) {
		return false, nil
	}
	if info.Signature == "" {
		return false, errors.New("release has no signature")
	}
	// Don't re-download a version we have already tried and failed to become.
	rec, ok := shouldAttempt(info.Version)
	if !ok {
		log.Printf("update: still on %s after %d attempts at %s; backing off until %s",
			current, rec.Count, info.Version, rec.At.Add(attemptCooldown).Format(time.RFC3339))
		return false, nil
	}
	data, err := get(ctx, info.URL)
	if err != nil {
		return false, err
	}
	if !verify(PinnedPublicKey, info.Signature, data) {
		return false, errors.New("release signature verification failed")
	}
	// Record before installing: the install restarts the service, so code after
	// it is not guaranteed to run.
	saveAttempt(rec)
	if err := install(data); err != nil {
		return false, err
	}
	return true, nil
}

func verify(pubB64, sigB64 string, data []byte) bool {
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

// isNewer reports whether candidate is a newer version than current using a
// dotted-numeric comparison (e.g. "0.2.0" > "0.1.9"). Non-numeric or unparsable
// versions fall back to string inequality.
func isNewer(candidate, current string) bool {
	c := parseVersion(candidate)
	cur := parseVersion(current)
	if c == nil || cur == nil {
		return candidate != current
	}
	for i := 0; i < len(c) && i < len(cur); i++ {
		if c[i] != cur[i] {
			return c[i] > cur[i]
		}
	}
	return len(c) > len(cur)
}

func parseVersion(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		out = append(out, n)
	}
	return out
}
