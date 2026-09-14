//go:build windows

package config

import (
	"os"
	"path/filepath"
)

func defaultDataDir() string {
	if pd := os.Getenv("ProgramData"); pd != "" {
		return filepath.Join(pd, "SystemCheck")
	}
	return `C:\ProgramData\SystemCheck`
}
