// Package screenshot captures the desktop on a randomized, policy-driven
// interval. Capture itself is platform-specific (see capture_windows.go); on
// unsupported platforms it is a no-op so the agent still builds and runs.
package screenshot

import (
	"context"
	crand "crypto/rand"
	"log"
	"math/rand"
	"time"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// Collector captures screenshots.
type Collector struct {
	pol wire.ScreenshotPolicy
}

// New returns a screenshot collector for the given policy.
func New(pol wire.ScreenshotPolicy) *Collector { return &Collector{pol: pol} }

// Name implements collectors.Collector.
func (c *Collector) Name() string { return "screenshot" }

// Start runs the randomized capture loop until ctx is cancelled.
func (c *Collector) Start(ctx context.Context, emit collectors.Emit, emitBlob collectors.EmitBlob) error {
	minS := c.pol.MinIntervalSec
	maxS := c.pol.MaxIntervalSec
	if minS <= 0 {
		minS = 180
	}
	if maxS < minS {
		maxS = minS
	}
	for {
		wait := time.Duration(minS) * time.Second
		if maxS > minS {
			wait += time.Duration(rand.Intn(maxS-minS+1)) * time.Second
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(wait):
		}
		if err := c.capture(ctx, emit, emitBlob); err != nil {
			log.Printf("screenshot: capture error: %v", err)
		}
	}
}

// randID returns a short random hex identifier for blob keys.
func randID() string {
	var b [16]byte
	_, _ = crand.Read(b[:])
	const hex = "0123456789abcdef"
	out := make([]byte, 32)
	for i, v := range b {
		out[i*2] = hex[v>>4]
		out[i*2+1] = hex[v&0x0f]
	}
	return string(out)
}
