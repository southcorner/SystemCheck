// Package installs detects newly installed software by diffing the Windows
// uninstall registry keys. The real implementation is Windows-only (see
// run_windows.go); on other platforms it is a no-op.
package installs

import (
	"context"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// Collector reports newly installed software.
type Collector struct {
	pol wire.Toggle
}

// New returns an installs collector.
func New(pol wire.Toggle) *Collector { return &Collector{pol: pol} }

// Name implements collectors.Collector.
func (c *Collector) Name() string { return "installs" }

// Start runs platform capture until ctx is cancelled.
func (c *Collector) Start(ctx context.Context, emit collectors.Emit, _ collectors.EmitBlob) error {
	return c.run(ctx, emit)
}
