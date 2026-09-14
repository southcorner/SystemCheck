// Package fswatch watches configured folders (default the user's Downloads) and
// emits a "download" event with the file name and size once a new/changed file
// settles. It uses fsnotify, which is cross-platform, so this collector is real
// on every platform. Browser download-history enrichment is platform-specific
// (see history_windows.go / history_other.go).
package fswatch

import (
	"context"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// settleDelay is how long a file must be quiet before we record it, so we don't
// capture a half-written download.
const settleDelay = 2 * time.Second

// Collector watches folders for new/changed files.
type Collector struct {
	pol wire.FswatchPolicy
}

// New returns a fswatch collector.
func New(pol wire.FswatchPolicy) *Collector { return &Collector{pol: pol} }

// Name implements collectors.Collector.
func (c *Collector) Name() string { return "fswatch" }

// Start watches the policy folders until ctx is cancelled.
func (c *Collector) Start(ctx context.Context, emit collectors.Emit, _ collectors.EmitBlob) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	for _, f := range c.pol.Folders {
		p := expandPath(f)
		if p == "" {
			continue
		}
		if err := w.Add(p); err != nil {
			log.Printf("fswatch: cannot watch %q: %v", p, err)
			continue
		}
		log.Printf("fswatch: watching %s", p)
	}

	var mu sync.Mutex
	timers := map[string]*time.Timer{}

	// Optional browser history enrichment on a slow poll.
	if c.pol.IncludeBrowserHistory {
		go c.browserHistoryLoop(ctx, emit)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-w.Events:
			if !ok {
				return nil
			}
			if event.Op&(fsnotify.Create|fsnotify.Write) == 0 {
				continue
			}
			path := event.Name
			if isTempDownload(path) {
				continue
			}
			mu.Lock()
			if t, exists := timers[path]; exists {
				t.Reset(settleDelay)
			} else {
				timers[path] = time.AfterFunc(settleDelay, func() {
					mu.Lock()
					delete(timers, path)
					mu.Unlock()
					emitFile(emit, path)
				})
			}
			mu.Unlock()
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			log.Printf("fswatch: %v", err)
		}
	}
}

func (c *Collector) browserHistoryLoop(ctx context.Context, emit collectors.Emit) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	scanBrowserHistory(emit) // once at startup
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			scanBrowserHistory(emit)
		}
	}
}

func emitFile(emit collectors.Emit, path string) {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() || fi.Size() == 0 {
		return
	}
	emit(wire.Event{
		Kind: "download",
		TS:   time.Now().UTC(),
		Data: map[string]interface{}{
			"name":   fi.Name(),
			"path":   path,
			"size":   fi.Size(),
			"folder": parentDir(path),
			"source": "filesystem",
		},
	})
}

// isTempDownload skips partial-download temp files created by browsers.
func isTempDownload(path string) bool {
	lower := strings.ToLower(path)
	for _, suffix := range []string{".crdownload", ".part", ".tmp", ".partial"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}
