//go:build !windows

package main

import (
	"context"

	"github.com/southcorner/systemcheck/agent/internal/config"
)

func platformRun(ctx context.Context, cfg *config.Config, _ bool) error {
	return runConsole(ctx, cfg)
}

func defaultConfigPathOS() string { return "/etc/systemcheck/agent.json" }
