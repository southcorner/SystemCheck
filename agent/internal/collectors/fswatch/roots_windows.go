//go:build windows

package fswatch

import (
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

// defaultRoots returns the trees to watch for downloads: the user's profile
// (covers Downloads, Desktop, Documents and any custom subfolder) plus any
// removable (USB) drives.
func defaultRoots() []string {
	var roots []string
	if up := os.Getenv("USERPROFILE"); up != "" {
		roots = append(roots, up)
	}
	if mask, err := windows.GetLogicalDrives(); err == nil {
		for i := 0; i < 26; i++ {
			if mask&(1<<uint(i)) == 0 {
				continue
			}
			root := string(rune('A'+i)) + `:\`
			p, err := windows.UTF16PtrFromString(root)
			if err != nil {
				continue
			}
			if windows.GetDriveType(p) == windows.DRIVE_REMOVABLE {
				roots = append(roots, root)
			}
		}
	}
	return roots
}

// excludeDir skips high-churn/system trees so the recursive watch stays light
// and quiet (downloads never land in these).
func excludeDir(path string) bool {
	lower := strings.ToLower(path)
	return strings.Contains(lower, `\appdata\`) ||
		strings.HasSuffix(lower, `\appdata`) ||
		strings.Contains(lower, `\$recycle.bin`) ||
		strings.Contains(lower, `\node_modules\`) ||
		strings.Contains(lower, `\windows\`)
}
