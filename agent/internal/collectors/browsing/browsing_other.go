//go:build !windows

package browsing

import "github.com/southcorner/systemcheck/agent/internal/collectors"

// scanAll is a no-op off Windows (browser profile locations are Windows-specific
// here; the fleet is Windows).
func scanAll(_ collectors.Emit) {}
