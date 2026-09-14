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
	"errors"
	"strconv"
	"strings"
)

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
	data, err := get(ctx, info.URL)
	if err != nil {
		return false, err
	}
	if !verify(PinnedPublicKey, info.Signature, data) {
		return false, errors.New("release signature verification failed")
	}
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
