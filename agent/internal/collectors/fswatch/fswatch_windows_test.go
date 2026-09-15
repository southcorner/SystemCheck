//go:build windows

package fswatch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// nonExcludedDir makes a temp dir that excludeDir won't skip (t.TempDir lives
// under AppData\Local\Temp, which the watcher deliberately ignores).
func nonExcludedDir(t *testing.T) string {
	t.Helper()
	base, err := os.MkdirTemp(".", "sctest")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	abs, err := filepath.Abs(base)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(abs) })
	if excludeDir(abs) {
		t.Fatalf("test dir unexpectedly excluded: %s", abs)
	}
	return abs
}

// Directly validate reading the Zone.Identifier alternate data stream.
func TestMOTWRead(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.bin")
	if err := os.WriteFile(f, []byte("data"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(f+":Zone.Identifier",
		[]byte("[ZoneTransfer]\r\nZoneId=3\r\nHostUrl=https://dl.example.org/a.bin\r\n"), 0o644); err != nil {
		t.Fatalf("write ADS: %v", err)
	}
	url, ok := isInternetDownload(f)
	if !ok {
		t.Fatal("expected MOTW file to be detected as internet download")
	}
	if domainOf(url) != "dl.example.org" {
		t.Fatalf("domain = %q, want dl.example.org", domainOf(url))
	}
	// A file with no ADS must not be detected.
	plain := filepath.Join(dir, "b.txt")
	os.WriteFile(plain, []byte("x"), 0o644)
	if _, ok := isInternetDownload(plain); ok {
		t.Fatal("plain file should not be an internet download")
	}
}

// End-to-end: a downloaded (MOTW) file in a watched tree is reported.
func TestFswatchEmitsInternetDownload(t *testing.T) {
	dir := nonExcludedDir(t)
	t.Setenv("USERPROFILE", dir)

	events := make(chan wire.Event, 4)
	c := New(wire.FswatchPolicy{Enabled: true})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = c.Start(ctx, func(e wire.Event) { events <- e }, nil) }()
	time.Sleep(600 * time.Millisecond)

	target := filepath.Join(dir, "invoice.pdf")
	if err := os.WriteFile(target, []byte("hello-pdf-bytes"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(target+":Zone.Identifier",
		[]byte("[ZoneTransfer]\r\nZoneId=3\r\nHostUrl=https://files.example.com/invoice.pdf\r\n"), 0o644); err != nil {
		t.Fatalf("write MOTW ADS: %v", err)
	}

	select {
	case e := <-events:
		if e.Kind != "download" || e.Data["name"] != "invoice.pdf" || e.Data["domain"] != "files.example.com" {
			t.Fatalf("unexpected event: %+v", e.Data)
		}
	case <-time.After(settleDelay + 5*time.Second):
		t.Fatal("timed out waiting for download event")
	}
}

// A plain local file (no Mark of the Web) must NOT be reported.
func TestFswatchIgnoresNonDownload(t *testing.T) {
	dir := nonExcludedDir(t)
	t.Setenv("USERPROFILE", dir)

	events := make(chan wire.Event, 4)
	c := New(wire.FswatchPolicy{Enabled: true})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = c.Start(ctx, func(e wire.Event) { events <- e }, nil) }()
	time.Sleep(600 * time.Millisecond)

	if err := os.WriteFile(filepath.Join(dir, "local-notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	select {
	case e := <-events:
		t.Fatalf("unexpected event for non-download file: %+v", e.Data)
	case <-time.After(settleDelay + 2*time.Second):
		// good: nothing emitted
	}
}
