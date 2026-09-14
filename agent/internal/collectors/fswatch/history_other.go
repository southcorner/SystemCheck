//go:build !windows

package fswatch

import "github.com/southcorner/systemcheck/agent/internal/collectors"

// scanBrowserHistory is a no-op on non-Windows platforms; browser-history
// enrichment is implemented for Windows (the supported endpoint OS).
func scanBrowserHistory(_ collectors.Emit) {}
