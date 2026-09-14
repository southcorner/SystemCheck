package fswatch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/southcorner/systemcheck/agent/internal/wire"
)

func TestFswatchEmitsDownload(t *testing.T) {
	dir := t.TempDir()
	events := make(chan wire.Event, 4)
	emit := func(e wire.Event) { events <- e }

	c := New(wire.FswatchPolicy{Enabled: true, Folders: []string{dir}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = c.Start(ctx, emit, nil) }()

	// Give the watcher time to register.
	time.Sleep(300 * time.Millisecond)

	target := filepath.Join(dir, "invoice.pdf")
	if err := os.WriteFile(target, []byte("hello-pdf-bytes"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	select {
	case e := <-events:
		if e.Kind != "download" {
			t.Fatalf("kind = %q, want download", e.Kind)
		}
		if e.Data["name"] != "invoice.pdf" {
			t.Fatalf("name = %v, want invoice.pdf", e.Data["name"])
		}
		if sz, _ := e.Data["size"].(int64); sz != int64(len("hello-pdf-bytes")) {
			t.Fatalf("size = %v, want %d", e.Data["size"], len("hello-pdf-bytes"))
		}
	case <-time.After(settleDelay + 3*time.Second):
		t.Fatal("timed out waiting for download event")
	}
}

func TestFswatchSkipsTempFiles(t *testing.T) {
	if !isTempDownload("C:\\Users\\x\\Downloads\\big.zip.crdownload") {
		t.Fatal("expected .crdownload to be skipped")
	}
	if isTempDownload("report.xlsx") {
		t.Fatal("did not expect .xlsx to be skipped")
	}
}
