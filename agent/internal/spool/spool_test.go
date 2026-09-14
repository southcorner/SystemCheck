package spool

import (
	"testing"
	"time"

	"github.com/southcorner/systemcheck/agent/internal/wire"
)

func TestSpoolEventRoundTrip(t *testing.T) {
	sp, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := sp.AddEvent(wire.Event{Kind: "foreground", TS: time.Now(), Data: map[string]interface{}{"process": "x.exe"}}); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if sp.Count() != 3 {
		t.Fatalf("expected 3 pending, got %d", sp.Count())
	}
	events, paths, err := sp.PendingEvents(10)
	if err != nil || len(events) != 3 {
		t.Fatalf("pending: n=%d err=%v", len(events), err)
	}
	sp.Remove(paths...)
	if sp.Count() != 0 {
		t.Fatalf("expected 0 after remove, got %d", sp.Count())
	}
}

func TestSpoolBlob(t *testing.T) {
	sp, _ := New(t.TempDir())
	if _, err := sp.AddBlob("shot.jpg", []byte("data")); err != nil {
		t.Fatalf("add blob: %v", err)
	}
	blobs, err := sp.PendingBlobs(10)
	if err != nil || len(blobs) != 1 || blobs[0].Key != "shot.jpg" {
		t.Fatalf("pending blobs: %+v err=%v", blobs, err)
	}
}
