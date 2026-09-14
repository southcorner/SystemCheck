//go:build windows

package enroll

import "golang.org/x/sys/windows"

func osVersion() string {
	v := windows.RtlGetVersion()
	return itoa(int(v.MajorVersion)) + "." + itoa(int(v.MinorVersion)) + "." + itoa(int(v.BuildNumber))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
