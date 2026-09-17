// Session-helper role: the part of the agent that runs inside the logged-in
// user's interactive session (launched by a per-user logon task), so the
// desktop-bound collectors (screenshots, foreground app) keep working when the
// privileged agent runs as a SYSTEM service in session 0.
//
// The helper holds no server credentials. It exchanges two small JSON files
// with the service through the (user-writable) spool directory:
//   - runtime.json  (service -> helper): machine id + effective policy
//   - session.json  (helper -> service): logged-in user + consent acceptance
// The service uploads whatever the helper drops in the spool and folds the
// session state into its heartbeat.
package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/southcorner/systemcheck/agent/internal/config"
	"github.com/southcorner/systemcheck/agent/internal/spool"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// currentInteractiveUser returns the process owner (e.g. "DOMAIN\\alice" on
// Windows), which for the session helper is the logged-in interactive user.
func currentInteractiveUser() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	if v := strings.TrimSpace(os.Getenv("USERNAME")); v != "" {
		if d := os.Getenv("USERDOMAIN"); d != "" {
			return d + `\` + v
		}
		return v
	}
	return "unknown"
}

func runtimePath(cfg *config.Config) string { return filepath.Join(cfg.SpoolDir(), "runtime.json") }
func sessionPath(cfg *config.Config) string { return filepath.Join(cfg.SpoolDir(), "session.json") }

// writeRuntime persists the machine id + current policy for the session helper.
// Best-effort: a helper simply waits until the file appears.
func writeRuntime(cfg *config.Config, machineID string, pol *wire.Policy) {
	if pol == nil {
		return
	}
	rt := wire.Runtime{MachineID: machineID, Policy: *pol}
	writeJSONFile(runtimePath(cfg), rt)
}

func readRuntime(cfg *config.Config) (*wire.Runtime, error) {
	b, err := os.ReadFile(runtimePath(cfg))
	if err != nil {
		return nil, err
	}
	var rt wire.Runtime
	if err := json.Unmarshal(b, &rt); err != nil {
		return nil, err
	}
	return &rt, nil
}

// readSessionState returns the helper's last-reported identity/consent, or a
// zero value if the helper has not written it yet.
func readSessionState(cfg *config.Config) wire.SessionState {
	var ss wire.SessionState
	if b, err := os.ReadFile(sessionPath(cfg)); err == nil {
		_ = json.Unmarshal(b, &ss)
	}
	return ss
}

func writeJSONFile(path string, v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return
	}
	// Atomic-ish write so a concurrent reader never sees a partial file.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// RunSessionAgent runs the desktop collectors in the current user session until
// ctx is cancelled. It requires no network access: the service does all server
// communication.
func RunSessionAgent(ctx context.Context, cfg *config.Config) error {
	sp, err := spool.New(cfg.SpoolDir())
	if err != nil {
		return fmt.Errorf("spool: %w", err)
	}

	user := currentInteractiveUser()
	log.Printf("session helper starting for user %q", user)
	sh := newHealth("session")

	// Consent click-through (once per user). Collection stays off until accepted.
	consented := hasLocalConsent(cfg, user)
	if !consented {
		if promptConsent(user) {
			consented = true
			_ = markLocalConsent(cfg, user)
			log.Printf("user %q accepted the monitoring notice", user)
		} else {
			log.Printf("user %q has not accepted the monitoring notice; collection stays off", user)
		}
	}
	// Publish identity + consent for the service's heartbeat.
	writeSessionState(cfg, user, consented, sh)

	for {
		if ctx.Err() != nil {
			return nil
		}
		writeSessionState(cfg, user, consented, sh)

		rt, err := readRuntime(cfg)
		active := err == nil && consented && rt.Policy.Active &&
			(rt.Policy.Screenshot.Enabled || rt.Policy.Foreground.Enabled ||
				rt.Policy.Fswatch.Enabled || rt.Policy.Browsing.Enabled)
		if !active {
			sh.reset()
			if !sleepCtx(ctx, 20*time.Second) {
				return nil
			}
			continue
		}
		// Run this policy generation until the policy changes or ctx is cancelled.
		runSessionGeneration(ctx, cfg, sp, rt, user, consented, sh)
	}
}

// runSessionGeneration runs the desktop collectors for one policy version and
// returns when the policy version changes or ctx is cancelled. Its cancel func
// is deferred in this scope, so collectors always stop cleanly.
func runSessionGeneration(ctx context.Context, cfg *config.Config, sp *spool.Spool, rt *wire.Runtime, user string, consented bool, sh *healthRegistry) {
	genCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	emit := func(e wire.Event) { _ = sp.AddEvent(e) }
	emitBlob := func(key string, data []byte) (string, error) {
		if _, err := sp.AddBlob(key, data); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s/%s", rt.MachineID, key), nil
	}
	startCollectors(genCtx, rt.Policy, RoleSession, emit, emitBlob, sh)

	for {
		writeSessionState(cfg, user, consented, sh) // keep the handoff fresh while running
		if !sleepCtx(ctx, 20*time.Second) {
			return
		}
		cur, err := readRuntime(cfg)
		if err != nil {
			continue
		}
		if cur.Policy.Version != rt.Policy.Version || cur.Policy.Active != rt.Policy.Active {
			return // caller re-evaluates and starts the next generation
		}
	}
}

// sleepCtx waits for d or until ctx is done; returns false if ctx was cancelled.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func writeSessionState(cfg *config.Config, user string, consented bool, sh *healthRegistry) {
	ss := wire.SessionState{
		InteractiveUser: user,
		Consented:       consented,
		UpdatedAt:       time.Now().UTC().Format(time.RFC3339),
	}
	if sh != nil {
		ss.Collectors = sh.snapshot()
	}
	writeJSONFile(sessionPath(cfg), ss)
}

// Local per-user consent marker so the click-through appears at most once per
// user on a shared machine.
func consentMarker(cfg *config.Config, user string) string {
	return filepath.Join(cfg.SpoolDir(), "consent-"+sanitizeUser(user)+".ack")
}
func hasLocalConsent(cfg *config.Config, user string) bool {
	_, err := os.Stat(consentMarker(cfg, user))
	return err == nil
}
func markLocalConsent(cfg *config.Config, user string) error {
	return os.WriteFile(consentMarker(cfg, user), []byte(time.Now().UTC().Format(time.RFC3339)), 0o644)
}

func sanitizeUser(u string) string {
	out := make([]rune, 0, len(u))
	for _, r := range u {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "unknown"
	}
	return string(out)
}
