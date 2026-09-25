// Package fswatch detects files DOWNLOADED FROM THE INTERNET, independent of
// which browser, browser profile, or download folder the user chose. It watches
// every local drive (all fixed + removable roots, plus any policy folders) and
// records a file only when it carries an internet "Mark of the Web"
// (Zone.Identifier ADS with ZoneId >= 3) - the tag Windows/SmartScreen attaches
// to internet downloads from every mainstream browser. That yields the file
// name, size, path and source domain regardless of the app that fetched it.
package fswatch

import (
	"context"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// settleDelay is how long a file must be quiet before we inspect it, so the
// download (and its Zone.Identifier stream, written last) has finished.
const settleDelay = 2 * time.Second

// maxWatchedDirs caps how many directories we register, so watching every local
// drive can't exhaust handles on a busy machine. Higher than the old
// profile-only cap because we now cover all fixed+removable drives; excludeDir
// prunes the big system trees so real machines stay well under this.
const maxWatchedDirs = 20000

// Collector watches for downloaded files.
type Collector struct {
	pol wire.FswatchPolicy
}

// New returns a fswatch collector.
func New(pol wire.FswatchPolicy) *Collector { return &Collector{pol: pol} }

// Name implements collectors.Collector.
func (c *Collector) Name() string { return "fswatch" }

// Start watches the roots until ctx is cancelled.
func (c *Collector) Start(ctx context.Context, emit collectors.Emit, _ collectors.EmitBlob) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	// Roots: user profile + removable drives (platform-derived) plus any
	// explicit folders configured in policy.
	roots := defaultRoots()
	for _, f := range c.pol.Folders {
		if p := expandPath(f); p != "" {
			roots = append(roots, p)
		}
	}
	watched := &dirCounter{}
	for _, r := range roots {
		addTree(w, r, watched)
	}
	log.Printf("fswatch: watching %d directories under %v (internet-download detection)", watched.n, roots)

	var mu sync.Mutex
	timers := map[string]*time.Timer{}

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-w.Events:
			if !ok {
				return nil
			}
			// New directory (e.g. a freshly created download subfolder): watch it too.
			if event.Op&fsnotify.Create != 0 {
				if fi, statErr := os.Stat(event.Name); statErr == nil && fi.IsDir() {
					addTree(w, event.Name, watched)
					continue
				}
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
					maybeEmitDownload(emit, path)
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

type dirCounter struct {
	n    int
	seen map[string]bool
}

// addTree registers dir and its subdirectories with the watcher, skipping
// high-churn/system trees, de-duplicating across overlapping roots (the profile
// lives under a drive root we also walk), and honouring the watch cap.
// Best-effort.
func addTree(w *fsnotify.Watcher, root string, c *dirCounter) {
	if c.seen == nil {
		c.seen = map[string]bool{}
	}
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entry: skip, keep walking
		}
		if !d.IsDir() {
			return nil
		}
		if excludeDir(p) {
			return filepath.SkipDir
		}
		key := strings.ToLower(filepath.Clean(p))
		if c.seen[key] {
			return filepath.SkipDir // already registered via an earlier root
		}
		if c.n >= maxWatchedDirs {
			return filepath.SkipDir
		}
		if w.Add(p) == nil {
			c.seen[key] = true
			c.n++
		}
		return nil
	})
}

// maybeEmitDownload records a settled file only if it is an internet download.
func maybeEmitDownload(emit collectors.Emit, path string) {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() || fi.Size() == 0 {
		return
	}
	src, ok := isInternetDownload(path)
	if !ok {
		return // ordinary file activity, not an internet download - ignore
	}
	if !markEmitted(path, fi.ModTime()) {
		return // already recorded this version of the file
	}
	emit(wire.Event{
		Kind: "download",
		TS:   time.Now().UTC(),
		Data: map[string]interface{}{
			"name":   fi.Name(),
			"path":   path,
			"size":   fi.Size(),
			"folder": filepath.Dir(path),
			"url":    src,
			"domain": domainOf(src),
			"source": "motw",
		},
	})
}

// Dedup so a file isn't reported repeatedly as it is written.
var (
	emitMu  sync.Mutex
	emitted = map[string]int64{}
)

func markEmitted(path string, mod time.Time) bool {
	emitMu.Lock()
	defer emitMu.Unlock()
	m := mod.UnixNano()
	if emitted[path] == m {
		return false
	}
	emitted[path] = m
	return true
}

func domainOf(raw string) string {
	if raw == "" {
		return ""
	}
	if u, err := url.Parse(raw); err == nil {
		return u.Hostname()
	}
	return ""
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
