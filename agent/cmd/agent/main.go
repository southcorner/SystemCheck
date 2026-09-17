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
	"time"
	_ "time/tzdata" // embedded tz database so Asia/Kolkata resolves without OS tzdata

	"github.com/southcorner/systemcheck/agent/internal/config"
	"github.com/southcorner/systemcheck/agent/internal/runner"
)

func main() {
	// Use IST for all local timestamps (log lines) regardless of the machine's
	// own timezone, so logs read consistently across the fleet. Event data is
	// still sent in UTC and rendered in IST by the dashboard.
	if loc, err := time.LoadLocation("Asia/Kolkata"); err == nil {
		time.Local = loc
	}
	log.SetPrefix("systemcheck-agent ")
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)

	cfgPath := flag.String("config", defaultConfigPath(), "path to agent config JSON")
	console := flag.Bool("console", false, "force console (foreground) mode")
	sessionAgent := flag.Bool("session-agent", false, "run the user-session helper (screenshots + foreground app) instead of the service; launched per-user at logon")
	flag.Parse()

	// Log to a file so failures are diagnosable when there is no console (the
	// service has no stdout). Start at the default data dir so even a config-load
	// error is captured; re-point if the config overrides the data dir.
	setupLogging(config.DefaultDataDir(), *sessionAgent)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if cfg.DataDir != config.DefaultDataDir() {
		setupLogging(cfg.DataDir, *sessionAgent)
	}

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

// setupLogging tees the log to a file (plus stderr) so the service's startup and
// runtime errors are visible without a console.
func setupLogging(dataDir string, sessionAgent bool) {
	path := filepath.Join(dataDir, "agent.log")
	if sessionAgent {
		path = filepath.Join(dataDir, "spool", "agent-session.log")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		log.Printf("log file %s: %v (continuing with stderr only)", path, err)
		return
	}
	// A Windows service has no valid stderr, so a direct MultiWriter(os.Stderr,f)
	// aborts on the stderr write and never reaches the file. Wrap stderr so its
	// errors are swallowed and the file is always written.
	log.SetOutput(io.MultiWriter(errIgnoringWriter{os.Stderr}, f))
	log.Printf("=== agent starting (session_helper=%v, pid=%d) ===", sessionAgent, os.Getpid())
}

// errIgnoringWriter forwards writes but never reports an error, so a dead
// stderr (service with no console) can't block a MultiWriter.
type errIgnoringWriter struct{ w io.Writer }

func (e errIgnoringWriter) Write(p []byte) (int, error) {
	_, _ = e.w.Write(p)
	return len(p), nil
}

// runConsole runs the agent in the foreground.
func runConsole(ctx context.Context, cfg *config.Config) error {
	return runner.Run(ctx, cfg)
}
