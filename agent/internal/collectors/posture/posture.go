// Package posture takes periodic device-posture snapshots (disk encryption,
// antivirus, firewall, patch level). The real implementation shells out to
// PowerShell on Windows (see run_windows.go); on other platforms it is a no-op.
package posture

import (
	"context"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// Collector reports device posture.
type Collector struct {
	pol wire.PosturePolicy
}

// New returns a posture collector.
func New(pol wire.PosturePolicy) *Collector { return &Collector{pol: pol} }

// Name implements collectors.Collector.
func (c *Collector) Name() string { return "posture" }

// Start runs platform capture until ctx is cancelled.
func (c *Collector) Start(ctx context.Context, emit collectors.Emit, _ collectors.EmitBlob) error {
	return c.run(ctx, emit)
}

func (c *Collector) intervalSec() int {
	if c.pol.IntervalSec <= 0 {
		return 3600
	}
	return c.pol.IntervalSec
}
