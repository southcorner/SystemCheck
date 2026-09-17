package runner

import (
	"sync"
	"time"

	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// sessionStaleAfter: if the helper's session.json is older than this, its
// collectors are treated as down (helper likely killed).
const sessionStaleAfter = 3 * time.Minute

// mergedCollectorHealth combines the service's own collector health with the
// session helper's (from the handoff file). If the helper hasn't reported
// recently, its policy-enabled collectors are reported DOWN so the server can
// flag a killed/closed helper.
func mergedCollectorHealth(svc *healthRegistry, ss wire.SessionState, pol *wire.Policy) []wire.CollectorStatus {
	out := svc.snapshot()
	if pol == nil || !pol.Active {
		return out // collection off: nothing is expected to run in-session
	}
	fresh := false
	if ss.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339, ss.UpdatedAt); err == nil && time.Since(t) < sessionStaleAfter {
			fresh = true
		}
	}
	if fresh {
		out = append(out, ss.Collectors...)
		return out
	}
	for _, name := range expectedSessionCollectors(pol) {
		out = append(out, wire.CollectorStatus{
			Name: name, Running: false, Error: "session helper not reporting", Role: "session",
		})
	}
	return out
}

func expectedSessionCollectors(pol *wire.Policy) []string {
	var n []string
	if pol.Screenshot.Enabled {
		n = append(n, "screenshot")
	}
	if pol.Foreground.Enabled {
		n = append(n, "foreground")
	}
	if pol.Fswatch.Enabled {
		n = append(n, "fswatch")
	}
	if pol.Browsing.Enabled {
		n = append(n, "browsing")
	}
	return n
}

// healthRegistry tracks the running state of the collectors in one process
// (service or session helper), so it can be reported to the server.
type healthRegistry struct {
	mu   sync.Mutex
	role string
	m    map[string]wire.CollectorStatus
}

func newHealth(role string) *healthRegistry {
	return &healthRegistry{role: role, m: map[string]wire.CollectorStatus{}}
}

// reset clears the registry (called when the collector set is (re)started).
func (h *healthRegistry) reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.m = map[string]wire.CollectorStatus{}
}

func (h *healthRegistry) set(name string, running bool, errMsg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.m[name] = wire.CollectorStatus{Name: name, Running: running, Error: errMsg, Role: h.role}
}

func (h *healthRegistry) snapshot() []wire.CollectorStatus {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]wire.CollectorStatus, 0, len(h.m))
	for _, v := range h.m {
		out = append(out, v)
	}
	return out
}
