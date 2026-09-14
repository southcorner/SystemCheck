//go:build windows

package usb

import (
	"context"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

const driveRemovable = 2 // DRIVE_REMOVABLE

func (c *Collector) run(ctx context.Context, emit collectors.Emit) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	known := map[string]bool{}
	// Seed the initial set silently so we only report changes.
	for _, d := range removableDrives() {
		known[d] = true
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		current := map[string]bool{}
		for _, d := range removableDrives() {
			current[d] = true
			if !known[d] {
				label, serial, fs := volumeInfo(d)
				emit(wire.Event{
					Kind: "usb",
					TS:   time.Now().UTC(),
					Data: map[string]interface{}{
						"action": "insert",
						"drive":  d,
						"label":  label,
						"serial": serial,
						"fs":     fs,
					},
				})
			}
		}
		for d := range known {
			if !current[d] {
				emit(wire.Event{
					Kind: "usb",
					TS:   time.Now().UTC(),
					Data: map[string]interface{}{"action": "remove", "drive": d},
				})
			}
		}
		known = current
	}
}

func removableDrives() []string {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil
	}
	var out []string
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		p, _ := syscall.UTF16PtrFromString(root)
		if windows.GetDriveType(p) == driveRemovable {
			out = append(out, root)
		}
	}
	return out
}

func volumeInfo(root string) (label, serial, fs string) {
	p, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return
	}
	labelBuf := make([]uint16, 261)
	fsBuf := make([]uint16, 261)
	var serialNum, maxComp, flags uint32
	err = windows.GetVolumeInformation(p,
		&labelBuf[0], uint32(len(labelBuf)),
		&serialNum, &maxComp, &flags,
		&fsBuf[0], uint32(len(fsBuf)))
	if err != nil {
		return
	}
	label = syscall.UTF16ToString(labelBuf)
	fs = syscall.UTF16ToString(fsBuf)
	serial = formatSerial(serialNum)
	return
}

func formatSerial(n uint32) string {
	const hexdigits = "0123456789ABCDEF"
	b := make([]byte, 9)
	b[4] = '-'
	b[0] = hexdigits[(n>>28)&0xf]
	b[1] = hexdigits[(n>>24)&0xf]
	b[2] = hexdigits[(n>>20)&0xf]
	b[3] = hexdigits[(n>>16)&0xf]
	b[5] = hexdigits[(n>>12)&0xf]
	b[6] = hexdigits[(n>>8)&0xf]
	b[7] = hexdigits[(n>>4)&0xf]
	b[8] = hexdigits[n&0xf]
	return string(b)
}
