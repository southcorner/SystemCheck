//go:build !windows

package fswatch

import (
	"os"
	"strings"
)

func defaultRoots() []string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return []string{h}
	}
	return nil
}

func excludeDir(path string) bool {
	for _, e := range []string{"/.cache/", "/node_modules/", "/.git/", "/Library/"} {
		if strings.Contains(path, e) {
			return true
		}
	}
	return false
}
