//go:build !windows

package foreground

// sample is a no-op on non-Windows platforms (the supported endpoint OS is
// Windows); it always reports idle so nothing is emitted.
func (c *Collector) sample() sampleResult {
	return sampleResult{idle: true}
}
