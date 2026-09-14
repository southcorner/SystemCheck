// Package printjobs records completed print jobs (document, printer, user,
// pages, size). The real implementation consumes the Windows ETW
// Microsoft-Windows-PrintService provider (see run_windows.go); on other
// platforms it is a no-op.
package printjobs

import (
	"context"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// Collector records print jobs.
type Collector struct {
	pol wire.Toggle
}

// New returns a printjobs collector.
func New(pol wire.Toggle) *Collector { return &Collector{pol: pol} }

// Name implements collectors.Collector.
func (c *Collector) Name() string { return "printjobs" }

// Start runs platform capture until ctx is cancelled.
func (c *Collector) Start(ctx context.Context, emit collectors.Emit, _ collectors.EmitBlob) error {
	return c.run(ctx, emit)
}
