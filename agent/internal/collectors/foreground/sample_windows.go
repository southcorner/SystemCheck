//go:build windows

package foreground

import (
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                    = windows.NewLazySystemDLL("user32.dll")
	procGetForegroundWindow   = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcID = user32.NewProc("GetWindowThreadProcessId")
	procGetWindowTextW        = user32.NewProc("GetWindowTextW")
	procGetLastInputInfo      = user32.NewProc("GetLastInputInfo")

	kernel32         = windows.NewLazySystemDLL("kernel32.dll")
	procGetTickCount = kernel32.NewProc("GetTickCount")
)

type lastInputInfo struct {
	cbSize uint32
	dwTime uint32
}

func (c *Collector) sample() sampleResult {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return sampleResult{idle: true}
	}

	// Window title.
	buf := make([]uint16, 512)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	title := syscall.UTF16ToString(buf)

	// Owning process id -> executable path.
	var pid uint32
	procGetWindowThreadProcID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	proc := processPath(pid)

	return sampleResult{process: proc, title: title, idle: c.isIdle()}
}

func processPath(pid uint32) string {
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
	return syscall.UTF16ToString(buf[:size])
}

func (c *Collector) isIdle() bool {
	threshold := c.pol.IdleThresholdSec
	if threshold <= 0 {
		return false
	}
	var lii lastInputInfo
	lii.cbSize = uint32(unsafe.Sizeof(lii))
	r, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&lii)))
	if r == 0 {
		return false
	}
	tick, _, _ := procGetTickCount.Call()
	idleMS := uint32(tick) - lii.dwTime
	return time.Duration(idleMS)*time.Millisecond > time.Duration(threshold)*time.Second
}
