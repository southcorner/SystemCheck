//go:build windows

package installs

import (
	"context"
	"time"

	"golang.org/x/sys/windows/registry"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

type uninstallLoc struct {
	root registry.Key
	path string
}

var locations = []uninstallLoc{
	{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
	{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
	{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
}

func (c *Collector) run(ctx context.Context, emit collectors.Emit) error {
	// Seed baseline silently so only new installs are reported.
	baseline := scan()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		current := scan()
		for name, ver := range current {
			if _, seen := baseline[name]; !seen {
				emit(wire.Event{
					Kind: "install",
					TS:   time.Now().UTC(),
					Data: map[string]interface{}{
						"name":    name,
						"version": ver,
						"action":  "added",
					},
				})
			}
		}
		baseline = current
	}
}

// scan returns a map of DisplayName -> DisplayVersion across all uninstall keys.
func scan() map[string]string {
	out := map[string]string{}
	for _, loc := range locations {
		k, err := registry.OpenKey(loc.root, loc.path, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		names, err := k.ReadSubKeyNames(-1)
		if err != nil {
			k.Close()
			continue
		}
		for _, sub := range names {
			s, err := registry.OpenKey(loc.root, loc.path+`\`+sub, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			name, _, _ := s.GetStringValue("DisplayName")
			ver, _, _ := s.GetStringValue("DisplayVersion")
			s.Close()
			if name != "" {
				out[name] = ver
			}
		}
		k.Close()
	}
	return out
}
