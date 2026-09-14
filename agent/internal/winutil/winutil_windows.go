//go:build windows

// Package winutil holds small Windows-only helpers shared by the ETW-based
// collectors (dns, netflow).
package winutil

import (
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/windows"
)

// ProcessName returns the base executable name for a pid, or "" if unavailable.
func ProcessName(pid uint32) string {
	if pid == 0 {
		return ""
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return ""
	}
	return filepath.Base(syscall.UTF16ToString(buf[:size]))
}

// ToInt64 coerces an ETW property value (often a string) to int64.
func ToInt64(v interface{}) int64 {
	switch n := v.(type) {
	case string:
		i, _ := strconv.ParseInt(n, 10, 64)
		return i
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	case uint32:
		return int64(n)
	default:
		return 0
	}
}

// Str coerces an ETW property value to string.
func Str(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
