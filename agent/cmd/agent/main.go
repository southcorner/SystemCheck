// Command agent is the SystemCheck endpoint agent. On Windows it runs as a
// service; on other platforms (and with -console) it runs in the foreground.
//
// Transparency: this agent is not covert. It shows a visible indicator while
// monitoring is active and only collects once consent is recorded server-side.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/southcorner/systemcheck/agent/internal/config"
	"github.com/southcorner/systemcheck/agent/internal/runner"
)

func main() {
	log.SetPrefix("systemcheck-agent ")
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)

	cfgPath := flag.String("config", defaultConfigPath(), "path to agent config JSON")
	console := flag.Bool("console", false, "force console (foreground) mode")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := platformRun(ctx, cfg, *console); err != nil {
		log.Fatalf("agent: %v", err)
	}
}

func defaultConfigPath() string {
	if v := os.Getenv("SC_CONFIG"); v != "" {
		return v
	}
	return defaultConfigPathOS()
}

// runConsole runs the agent in the foreground.
func runConsole(ctx context.Context, cfg *config.Config) error {
	return runner.Run(ctx, cfg)
}
