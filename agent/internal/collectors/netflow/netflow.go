// Package netflow accounts per-process network bytes sent/received over a rollup
// window. The real implementation consumes the Windows ETW
// Microsoft-Windows-Kernel-Network provider (see run_windows.go); on other
// platforms it is a no-op.
package netflow

import (
	"context"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// Collector aggregates per-process network byte counts.
type Collector struct {
	pol wire.NetflowPolicy
}

// New returns a netflow collector.
func New(pol wire.NetflowPolicy) *Collector { return &Collector{pol: pol} }

// Name implements collectors.Collector.
func (c *Collector) Name() string { return "netflow" }

// Start runs the platform capture until ctx is cancelled.
func (c *Collector) Start(ctx context.Context, emit collectors.Emit, _ collectors.EmitBlob) error {
	return c.run(ctx, emit)
}

func (c *Collector) rollupSec() int {
	if c.pol.RollupSec <= 0 {
		return 60
	}
	return c.pol.RollupSec
}
