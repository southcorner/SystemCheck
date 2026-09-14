// Package foreground tracks the active foreground application and window title,
// gating on user idle time. Sampling is platform-specific.
package foreground

import (
	"context"
	"path/filepath"
	"time"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// Collector samples the foreground application.
type Collector struct {
	pol wire.ForegroundPolicy
}

// New returns a foreground collector.
func New(pol wire.ForegroundPolicy) *Collector { return &Collector{pol: pol} }

// Name implements collectors.Collector.
func (c *Collector) Name() string { return "foreground" }

// sample result.
type sampleResult struct {
	process string
	title   string
	idle    bool
}

// Start polls the foreground window and emits per-process active-time events.
func (c *Collector) Start(ctx context.Context, emit collectors.Emit, _ collectors.EmitBlob) error {
	poll := c.pol.PollSec
	if poll <= 0 {
		poll = 5
	}
	ticker := time.NewTicker(time.Duration(poll) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		s := c.sample()
		if s.idle || s.process == "" {
			continue
		}
		emit(wire.Event{
			Kind: "foreground",
			TS:   time.Now().UTC(),
			Data: map[string]interface{}{
				"process":    filepath.Base(s.process),
				"path":       s.process,
				"title":      s.title,
				"active_sec": poll,
			},
		})
	}
}
