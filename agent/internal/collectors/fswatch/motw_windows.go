//go:build windows

package fswatch

import (
	"os"
	"strconv"
	"strings"
)

// isInternetDownload reports whether path carries an internet Mark of the Web -
// the Zone.Identifier alternate data stream that Windows/SmartScreen attaches to
// files fetched from the internet (ZoneId 3 = Internet, 4 = Restricted). It
// returns the recorded source URL (HostUrl, else ReferrerUrl) when present.
func isInternetDownload(path string) (sourceURL string, yes bool) {
	b, err := os.ReadFile(path + ":Zone.Identifier")
	if err != nil {
		return "", false // no MOTW stream -> not an internet download
	}
	zone := -1
	var host, referrer string
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "zoneid":
			zone, _ = strconv.Atoi(strings.TrimSpace(v))
		case "hosturl":
			host = strings.TrimSpace(v)
		case "referrerurl":
			referrer = strings.TrimSpace(v)
		}
	}
	if zone < 3 {
		return "", false
	}
	if host != "" {
		return host, true
	}
	return referrer, true
}
