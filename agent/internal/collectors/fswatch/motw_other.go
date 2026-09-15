//go:build !windows

package fswatch

// isInternetDownload is Windows-only (Mark of the Web is a Windows feature).
// On other platforms nothing is treated as an internet download.
func isInternetDownload(path string) (string, bool) { return "", false }
