//go:build windows

package runner

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// promptConsent shows the monitoring notice in the user's session and returns
// true only if the user explicitly accepts. Collection stays off until then, in
// keeping with the app's transparency/consent design.
func promptConsent(user string) bool {
	const (
		mbYesNo          = 0x00000004
		mbIconInfo       = 0x00000040
		mbSystemModal    = 0x00001000
		mbSetForeground  = 0x00010000
		idYes            = 6
	)
	text := "This device is monitored by SystemCheck under your organization's " +
		"monitoring policy while you are signed in as " + user + ".\r\n\r\n" +
		"Monitoring may include periodic screenshots, active-application usage, " +
		"and network/security telemetry. A visible indicator shows when it is active.\r\n\r\n" +
		"Click Yes to acknowledge and consent. Monitoring will NOT start until you accept."
	title := "SystemCheck — Monitoring Notice"

	tp, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return false
	}
	cp, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return false
	}
	user32 := windows.NewLazySystemDLL("user32.dll")
	ret, _, _ := user32.NewProc("MessageBoxW").Call(
		0,
		uintptr(unsafe.Pointer(tp)),
		uintptr(unsafe.Pointer(cp)),
		uintptr(mbYesNo|mbIconInfo|mbSystemModal|mbSetForeground),
	)
	return ret == idYes
}
