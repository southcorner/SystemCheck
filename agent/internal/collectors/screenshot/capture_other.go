//go:build !windows

package screenshot

import (
	"context"
	"sync"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
)

var warnOnce sync.Once

// capture is a no-op on non-Windows platforms (the supported endpoint OS is
// Windows). It exists so the agent builds and runs cross-platform for testing.
func (c *Collector) capture(ctx context.Context, emit collectors.Emit, emitBlob collectors.EmitBlob) error {
	warnOnce.Do(func() {
		// Logged once; capture is only implemented on Windows.
	})
	return nil
}
