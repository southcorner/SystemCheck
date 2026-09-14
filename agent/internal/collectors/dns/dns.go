// Package dns records DNS queries (domains accessed) with the process that made
// them. The real implementation consumes the Windows ETW
// Microsoft-Windows-DNS-Client provider (see run_windows.go); on other
// platforms it is a no-op.
package dns

import (
	"context"
	"path"
	"strings"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// Collector records DNS queries.
type Collector struct {
	pol  wire.Toggle
	excl wire.Exclusions
}

// New returns a dns collector. Exclusions are applied locally so excluded
// domains/processes are never sent to the server.
func New(pol wire.Toggle, excl wire.Exclusions) *Collector {
	return &Collector{pol: pol, excl: excl}
}

// Name implements collectors.Collector.
func (c *Collector) Name() string { return "dns" }

// Start runs the platform capture until ctx is cancelled.
func (c *Collector) Start(ctx context.Context, emit collectors.Emit, _ collectors.EmitBlob) error {
	return c.run(ctx, emit)
}

// excluded reports whether a (domain, process) pair should be dropped per policy.
func (c *Collector) excluded(domain, process string) bool {
	domain = strings.ToLower(strings.TrimSuffix(domain, "."))
	for _, p := range c.excl.Processes {
		if strings.EqualFold(p, process) {
			return true
		}
	}
	for _, d := range c.excl.Domains {
		d = strings.ToLower(d)
		if d == domain {
			return true
		}
		if ok, _ := path.Match(d, domain); ok {
			return true
		}
		// suffix form ".example.com" or "*.example.com"
		if strings.HasPrefix(d, "*.") && strings.HasSuffix(domain, d[1:]) {
			return true
		}
	}
	return false
}
