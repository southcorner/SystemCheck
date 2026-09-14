package store

import (
	"testing"
	"time"
)

func TestRetentionCutoffs(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	c := RetentionCutoffs(now, 30, 90, 180, 365)
	if got := c.Screenshots; !got.Equal(now.AddDate(0, 0, -30)) {
		t.Fatalf("screenshots cutoff = %v", got)
	}
	if got := c.Activity; !got.Equal(now.AddDate(0, 0, -90)) {
		t.Fatalf("activity cutoff = %v", got)
	}
	if got := c.Audit; !got.Equal(now.AddDate(0, 0, -365)) {
		t.Fatalf("audit cutoff = %v", got)
	}
	// Zero/negative window disables that class.
	c = RetentionCutoffs(now, 0, -5, 180, 365)
	if !c.Screenshots.IsZero() {
		t.Fatal("expected zero screenshots cutoff when window is 0")
	}
	if !c.Activity.IsZero() {
		t.Fatal("expected zero activity cutoff when window is negative")
	}
}
