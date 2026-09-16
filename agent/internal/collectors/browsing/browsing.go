// Package browsing harvests visited-URL history from the user's browsers
// (Chromium family + Firefox, all profiles) and emits "visit" events with the
// URL, page title, domain, browser and time - far more precise than DNS for
// spotting where someone actually went. It runs in the user-session helper
// (the history lives in the user's profile) and polls periodically.
package browsing

import (
	"context"
	"time"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

const scanInterval = 5 * time.Minute

// Collector reads browser history on an interval.
type Collector struct {
	pol wire.Toggle
}

// New returns a browsing-history collector.
func New(pol wire.Toggle) *Collector { return &Collector{pol: pol} }

// Name implements collectors.Collector.
func (c *Collector) Name() string { return "browsing" }

// Start scans browser history until ctx is cancelled.
func (c *Collector) Start(ctx context.Context, emit collectors.Emit, _ collectors.EmitBlob) error {
	scanAll(emit) // once at startup
	t := time.NewTicker(scanInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			scanAll(emit)
		}
	}
}
