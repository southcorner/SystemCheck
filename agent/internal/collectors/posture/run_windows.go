//go:build windows

package posture

import (
	"context"
	"encoding/json"
	"os/exec"
	"time"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// postureScript emits a compact JSON object describing device posture. Each probe
// is wrapped in try/catch so a missing feature does not fail the whole snapshot.
const postureScript = `
$r=[ordered]@{}
try{$d=Get-MpComputerStatus;$r.defender_enabled=[bool]$d.AntivirusEnabled;$r.realtime=[bool]$d.RealTimeProtectionEnabled;$r.sig_age_days=[int]$d.AntivirusSignatureAge}catch{}
try{$bl=Get-BitLockerVolume -MountPoint $env:SystemDrive;$r.bitlocker=$bl.ProtectionStatus.ToString()}catch{}
try{$fw=Get-NetFirewallProfile;$r.firewall_on=[bool](($fw|Where-Object{$_.Enabled -eq $true}|Measure-Object).Count -gt 0)}catch{}
try{$h=Get-HotFix|Sort-Object InstalledOn -Descending|Select-Object -First 1;$r.last_hotfix=$h.HotFixID}catch{}
$r|ConvertTo-Json -Compress
`

func (c *Collector) run(ctx context.Context, emit collectors.Emit) error {
	interval := time.Duration(c.intervalSec()) * time.Second
	// Take an initial snapshot, then repeat on the interval.
	c.snapshot(ctx, emit)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			c.snapshot(ctx, emit)
		}
	}
}

func (c *Collector) snapshot(ctx context.Context, emit collectors.Emit) {
	cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", postureScript).Output()
	if err != nil {
		return
	}
	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil || data == nil {
		return
	}
	emit(wire.Event{Kind: "posture", TS: time.Now().UTC(), Data: data})
}
