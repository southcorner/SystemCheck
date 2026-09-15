// Command agent is the SystemCheck endpoint agent. On Windows it runs as a
// service; on other platforms (and with -console) it runs in the foreground.
//
// Transparency: this agent is not covert. It shows a visible indicator while
// monitoring is active and only collects once consent is recorded server-side.
package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/southcorner/systemcheck/agent/internal/config"
	"github.com/southcorner/systemcheck/agent/internal/runner"
)

func main() {
	log.SetPrefix("systemcheck-agent ")
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)

	cfgPath := flag.String("config", defaultConfigPath(), "path to agent config JSON")
	console := flag.Bool("console", false, "force console (foreground) mode")
	sessionAgent := flag.Bool("session-agent", false, "run the user-session helper (screenshots + foreground app) instead of the service; launched per-user at logon")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Log to a file so failures are diagnosable when there is no console (the
	// service has no stdout). Service/console -> data dir; the unprivileged
	// session helper -> the user-writable spool dir.
	setupLogging(cfg, *sessionAgent)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// The user-session helper runs in the logged-in user's session (not as a
	// service) and does the desktop-bound collection.
	if *sessionAgent {
		if err := runner.RunSessionAgent(ctx, cfg); err != nil {
			log.Fatalf("session agent: %v", err)
		}
		return
	}

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

// setupLogging tees the log to a rolling-ish file (in addition to stderr) so
// the service's startup and runtime errors are visible without a console.
func setupLogging(cfg *config.Config, sessionAgent bool) {
	path := filepath.Join(cfg.DataDir, "agent.log")
	if sessionAgent {
		path = filepath.Join(cfg.SpoolDir(), "agent-session.log")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		log.Printf("log file %s: %v (continuing with stderr only)", path, err)
		return
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	log.Printf("=== agent starting (session_helper=%v, pid=%d) ===", sessionAgent, os.Getpid())
}

// runConsole runs the agent in the foreground.
func runConsole(ctx context.Context, cfg *config.Config) error {
	return runner.Run(ctx, cfg)
}
