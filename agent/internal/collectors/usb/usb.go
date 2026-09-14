// Package usb detects removable-media (USB drive) insertion and removal. The
// real implementation polls logical drives on Windows (see usb_windows.go); on
// other platforms it is a no-op.
package usb

import (
	"context"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// Collector watches for removable-media changes.
type Collector struct {
	pol wire.Toggle
}

// New returns a usb collector.
func New(pol wire.Toggle) *Collector { return &Collector{pol: pol} }

// Name implements collectors.Collector.
func (c *Collector) Name() string { return "usb" }

// Start runs platform capture until ctx is cancelled.
func (c *Collector) Start(ctx context.Context, emit collectors.Emit, _ collectors.EmitBlob) error {
	return c.run(ctx, emit)
}
