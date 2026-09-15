//go:build !windows

package runner

// promptConsent has no interactive GUI off Windows (the production consent
// click-through is Windows-only). Returning true keeps the dev/console path
// usable; real employee consent is captured by the Windows session helper.
func promptConsent(user string) bool { return true }
