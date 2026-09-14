//go:build windows

package main

import (
	"context"
	"os"

	"golang.org/x/sys/windows/svc"

	"github.com/southcorner/systemcheck/agent/internal/config"
	"github.com/southcorner/systemcheck/agent/internal/runner"
)

// handler implements svc.Handler, bridging the SCM lifecycle to runner.Run.
type handler struct {
	ctx context.Context
	cfg *config.Config
}

func (h *handler) Execute(_ []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown
	s <- svc.Status{State: svc.StartPending}

	runCtx, cancel := context.WithCancel(h.ctx)
	defer cancel()
	go func() {
		if err := runner.Run(runCtx, h.cfg); err != nil {
			// Service will report stopped; details go to the log.
		}
	}()

	s <- svc.Status{State: svc.Running, Accepts: accepted}
	for req := range r {
		switch req.Cmd {
		case svc.Interrogate:
			s <- req.CurrentStatus
		case svc.Stop, svc.Shutdown:
			s <- svc.Status{State: svc.StopPending}
			cancel()
			return false, 0
		}
	}
	return false, 0
}

func getenv(k string) string { return os.Getenv(k) }
