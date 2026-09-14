//go:build windows

package main

import (
	"context"
	"path/filepath"

	"golang.org/x/sys/windows/svc"

	"github.com/southcorner/systemcheck/agent/internal/config"
)

const serviceName = "SystemCheckAgent"

// platformRun runs as a Windows service when launched by the SCM, otherwise in
// the console.
func platformRun(ctx context.Context, cfg *config.Config, console bool) error {
	if !console {
		isService, err := svc.IsWindowsService()
		if err == nil && isService {
			return svc.Run(serviceName, &handler{ctx: ctx, cfg: cfg})
		}
	}
	return runConsole(ctx, cfg)
}

func defaultConfigPathOS() string {
	if pd := envProgramData(); pd != "" {
		return filepath.Join(pd, "SystemCheck", "agent.json")
	}
	return `C:\ProgramData\SystemCheck\agent.json`
}

func envProgramData() string {
	return getenv("ProgramData")
}
