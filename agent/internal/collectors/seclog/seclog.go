// Package seclog records selected Windows Security event-log entries (sign-ins,
// failed sign-ins, special-privilege assignments). The real implementation
// shells to wevtutil on Windows (see run_windows.go); on other platforms it is a
// no-op.
package seclog

import (
	"context"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// Collector records Security event-log entries.
type Collector struct {
	pol wire.Toggle
}

// New returns a seclog collector.
func New(pol wire.Toggle) *Collector { return &Collector{pol: pol} }

// Name implements collectors.Collector.
func (c *Collector) Name() string { return "seclog" }

// Start runs platform capture until ctx is cancelled.
func (c *Collector) Start(ctx context.Context, emit collectors.Emit, _ collectors.EmitBlob) error {
	return c.run(ctx, emit)
}
