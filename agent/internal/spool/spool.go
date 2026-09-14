// Package spool is a durable on-disk queue that buffers events and screenshot
// blobs when the server is unreachable. It is a simple, dependency-free
// filesystem queue: each item is a file, drained in filename (time) order.
package spool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// Spool buffers events and blobs on disk.
type Spool struct {
	dir string
	mu  sync.Mutex
	seq int64
}

// New creates a spool rooted at dir.
func New(dir string) (*Spool, error) {
	if err := os.MkdirAll(filepath.Join(dir, "events"), 0o750); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "blobs"), 0o750); err != nil {
		return nil, err
	}
	return &Spool{dir: dir}, nil
}

func (s *Spool) nextName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	return fmt.Sprintf("%020d-%06d", time.Now().UnixNano(), s.seq)
}

// AddEvent appends an event to the queue.
func (s *Spool) AddEvent(e wire.Event) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	name := filepath.Join(s.dir, "events", s.nextName()+".json")
	return os.WriteFile(name, b, 0o640)
}

// AddBlob stores a screenshot blob under a key and returns the stored path.
func (s *Spool) AddBlob(key string, data []byte) (string, error) {
	name := filepath.Join(s.dir, "blobs", key)
	if err := os.MkdirAll(filepath.Dir(name), 0o750); err != nil {
		return "", err
	}
	return name, os.WriteFile(name, data, 0o640)
}

// PendingEvents returns up to max queued events with their file paths so the
// caller can delete them after a successful send.
func (s *Spool) PendingEvents(max int) (events []wire.Event, paths []string, err error) {
	dir := filepath.Join(s.dir, "events")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		if len(events) >= max {
			break
		}
		p := filepath.Join(dir, n)
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			continue
		}
		var ev wire.Event
		if json.Unmarshal(b, &ev) != nil {
			continue
		}
		events = append(events, ev)
		paths = append(paths, p)
	}
	return events, paths, nil
}

// Remove deletes queued files by path (after a successful send).
func (s *Spool) Remove(paths ...string) {
	for _, p := range paths {
		_ = os.Remove(p)
	}
}

// Count returns the number of pending events.
func (s *Spool) Count() int {
	entries, err := os.ReadDir(filepath.Join(s.dir, "events"))
	if err != nil {
		return 0
	}
	return len(entries)
}

// BlobItem is a pending screenshot blob.
type BlobItem struct {
	Key  string
	Path string
}

// PendingBlobs lists up to max spooled blobs (by key = filename).
func (s *Spool) PendingBlobs(max int) ([]BlobItem, error) {
	dir := filepath.Join(s.dir, "blobs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []BlobItem
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		out = append(out, BlobItem{Key: e.Name(), Path: filepath.Join(dir, e.Name())})
		if len(out) >= max {
			break
		}
	}
	return out, nil
}
