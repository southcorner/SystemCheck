// Package runner wires enrollment, policy, collectors, the spool, and the
// transport into the agent's main run loop.
package runner

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
	"github.com/southcorner/systemcheck/agent/internal/collectors/dns"
	"github.com/southcorner/systemcheck/agent/internal/collectors/foreground"
	"github.com/southcorner/systemcheck/agent/internal/collectors/fswatch"
	"github.com/southcorner/systemcheck/agent/internal/collectors/installs"
	"github.com/southcorner/systemcheck/agent/internal/collectors/netflow"
	"github.com/southcorner/systemcheck/agent/internal/collectors/posture"
	"github.com/southcorner/systemcheck/agent/internal/collectors/printjobs"
	"github.com/southcorner/systemcheck/agent/internal/collectors/screenshot"
	"github.com/southcorner/systemcheck/agent/internal/collectors/seclog"
	"github.com/southcorner/systemcheck/agent/internal/collectors/usb"
	"github.com/southcorner/systemcheck/agent/internal/config"
	"github.com/southcorner/systemcheck/agent/internal/enroll"
	"github.com/southcorner/systemcheck/agent/internal/spool"
	"github.com/southcorner/systemcheck/agent/internal/transport"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// Version is the agent version string.
const Version = "0.1.0"

// Run executes the agent until ctx is cancelled.
func Run(ctx context.Context, cfg *config.Config) error {
	if cfg.ServerURL == "" {
		return fmt.Errorf("server_url not configured")
	}

	machineID, err := enroll.Run(ctx, cfg, Version)
	if err != nil {
		return fmt.Errorf("enroll: %w", err)
	}
	log.Printf("agent enrolled as machine %s", machineID)

	client, err := transport.NewMTLSClient(cfg.ServerURL, cfg.CertPath(), cfg.KeyPath(), cfg.CAPath())
	if err != nil {
		return fmt.Errorf("client: %w", err)
	}

	sp, err := spool.New(cfg.SpoolDir())
	if err != nil {
		return fmt.Errorf("spool: %w", err)
	}

	emit := func(e wire.Event) {
		if err := sp.AddEvent(e); err != nil {
			log.Printf("spool event: %v", err)
		}
	}
	emitBlob := func(key string, data []byte) (string, error) {
		if _, err := sp.AddBlob(key, data); err != nil {
			return "", err
		}
		// The server namespaces uploads under the machine id, so the metadata
		// event must reference the same key it will be stored under.
		return fmt.Sprintf("%s/%s", machineID, key), nil
	}

	// Fetch the initial policy (retry until reachable).
	pol := waitForPolicy(ctx, client)
	if pol == nil {
		return ctx.Err()
	}
	log.Printf("policy v%d active=%v", pol.Version, pol.Active)

	// Show the transparency indicator (see indicator_*.go).
	showIndicator(pol.Active)

	collectorCtx, cancelCollectors := context.WithCancel(ctx)
	if pol.Active {
		startCollectors(collectorCtx, *pol, emit, emitBlob)
	}

	drainTicker := time.NewTicker(15 * time.Second)
	policyTicker := time.NewTicker(5 * time.Minute)
	hbEvery := time.Duration(pol.HeartbeatSec) * time.Second
	if hbEvery <= 0 {
		hbEvery = 60 * time.Second
	}
	hbTicker := time.NewTicker(hbEvery)
	defer drainTicker.Stop()
	defer policyTicker.Stop()
	defer hbTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			cancelCollectors()
			return nil
		case <-drainTicker.C:
			drain(ctx, client, sp)
		case <-hbTicker.C:
			_ = client.Heartbeat(ctx, wire.Heartbeat{
				AgentVersion: Version, PolicyVersion: pol.Version,
				QueuedEvents: sp.Count(), Healthy: true,
			})
		case <-policyTicker.C:
			newPol, err := client.GetPolicy(ctx)
			if err != nil {
				continue
			}
			if newPol.Version != pol.Version || newPol.Active != pol.Active {
				log.Printf("policy changed: v%d active=%v", newPol.Version, newPol.Active)
				cancelCollectors()
				collectorCtx, cancelCollectors = context.WithCancel(ctx)
				showIndicator(newPol.Active)
				if newPol.Active {
					startCollectors(collectorCtx, *newPol, emit, emitBlob)
				}
				pol = newPol
			}
		}
	}
}

func startCollectors(ctx context.Context, pol wire.Policy, emit collectors.Emit, emitBlob collectors.EmitBlob) {
	var active []collectors.Collector
	if pol.Screenshot.Enabled {
		active = append(active, screenshot.New(pol.Screenshot))
	}
	if pol.Foreground.Enabled {
		active = append(active, foreground.New(pol.Foreground))
	}
	if pol.DNS.Enabled {
		active = append(active, dns.New(pol.DNS, pol.Exclusions))
	}
	if pol.Netflow.Enabled {
		active = append(active, netflow.New(pol.Netflow))
	}
	if pol.Fswatch.Enabled {
		active = append(active, fswatch.New(pol.Fswatch))
	}
	if pol.USB.Enabled {
		active = append(active, usb.New(pol.USB))
	}
	if pol.PrintJobs.Enabled {
		active = append(active, printjobs.New(pol.PrintJobs))
	}
	if pol.Installs.Enabled {
		active = append(active, installs.New(pol.Installs))
	}
	if pol.Posture.Enabled {
		active = append(active, posture.New(pol.Posture))
	}
	if pol.Seclog.Enabled {
		active = append(active, seclog.New(pol.Seclog))
	}
	for _, c := range active {
		c := c
		go func() {
			if err := c.Start(ctx, emit, emitBlob); err != nil {
				log.Printf("collector %s stopped: %v", c.Name(), err)
			}
		}()
	}
	log.Printf("started %d collectors", len(active))
}

// drain uploads pending blobs, then sends pending events.
func drain(ctx context.Context, client *transport.Client, sp *spool.Spool) {
	blobs, _ := sp.PendingBlobs(50)
	for _, b := range blobs {
		data, err := os.ReadFile(b.Path)
		if err != nil {
			continue
		}
		if _, err := client.UploadScreenshot(ctx, b.Key, data); err != nil {
			log.Printf("upload blob: %v", err)
			return // server unreachable; try again next tick
		}
		sp.Remove(b.Path)
	}

	events, paths, err := sp.PendingEvents(500)
	if err != nil || len(events) == 0 {
		return
	}
	batch := wire.IngestBatch{
		BatchID: fmt.Sprintf("%d", time.Now().UnixNano()),
		SentAt:  time.Now().UTC(),
		Events:  events,
	}
	if err := client.Ingest(ctx, batch); err != nil {
		log.Printf("ingest: %v", err)
		return
	}
	sp.Remove(paths...)
}

func waitForPolicy(ctx context.Context, client *transport.Client) *wire.Policy {
	backoff := 2 * time.Second
	for {
		pol, err := client.GetPolicy(ctx)
		if err == nil {
			return pol
		}
		log.Printf("policy fetch failed: %v (retrying in %s)", err, backoff)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		if backoff < 60*time.Second {
			backoff *= 2
		}
	}
}
