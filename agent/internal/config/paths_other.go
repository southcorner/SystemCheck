//go:build !windows

package config

func defaultDataDir() string {
	return "/var/lib/systemcheck"
}
