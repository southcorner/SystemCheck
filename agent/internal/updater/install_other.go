//go:build !windows

package updater

import "errors"

// Install is unsupported off Windows (the supported endpoint OS).
func Install(_ []byte) error { return errors.New("self-update is only implemented on Windows") }
