//go:build windows

package fswatch

import (
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

// defaultRoots returns the trees to watch for downloads: every local drive,
// fixed and removable (C:\, D:\, USB sticks, ...). Employees may save downloads
// anywhere - not just under their profile - so we cover all local roots and let
// the MOTW check (only internet-downloaded files carry a Zone.Identifier) plus
// excludeDir keep the signal clean. Network drives are intentionally skipped:
// recursive change-watching over SMB is unreliable and heavy.
//
// The user's profile is listed first so it is always registered before the
// per-directory watch cap can be reached (the drive walk that follows dedups
// against it), guaranteeing Downloads/Desktop/Documents are never starved by an
// unrelated large tree elsewhere on the disk.
func defaultRoots() []string {
	var roots []string
	if up := os.Getenv("USERPROFILE"); up != "" {
		roots = append(roots, up)
	}
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return roots // at least the profile, if drive enumeration failed
	}
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		p, err := windows.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		switch windows.GetDriveType(p) {
		case windows.DRIVE_FIXED, windows.DRIVE_REMOVABLE:
			roots = append(roots, root)
		}
	}
	return roots
}

// excludeDir skips high-churn/system trees so the whole-drive watch stays light
// and quiet. Downloads never land in these, and pruning them keeps the
// per-directory watch cap available for the trees users actually save into.
func excludeDir(path string) bool {
	lower := strings.ToLower(path)
	for _, frag := range []string{
		`\appdata\`,
		`\$recycle.bin`,
		`\node_modules\`,
		`\windows\`,
		`\windows.old\`,
		`\program files\`,
		`\program files (x86)\`,
		`\programdata\`,
		`\system volume information`,
		`\$windows.~bt`,
		`\$windows.~ws`,
	} {
		if strings.Contains(lower, frag) {
			return true
		}
	}
	// Bare drive-root system dirs (no trailing separator in the walked path).
	switch {
	case strings.HasSuffix(lower, `\appdata`),
		strings.HasSuffix(lower, `\windows`),
		strings.HasSuffix(lower, `\program files`),
		strings.HasSuffix(lower, `\program files (x86)`),
		strings.HasSuffix(lower, `\programdata`):
		return true
	}
	return false
}
