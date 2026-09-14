// Package collectors defines the Collector interface implemented by each signal
// source (screenshot, foreground app, dns, netflow, fswatch, ...).
package collectors

import (
	"context"

	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// Emit is called by a collector to hand an event to the agent pipeline.
type Emit func(wire.Event)

// EmitBlob is called by a collector to store a large binary payload (e.g. a
// screenshot). It returns the object key to reference in the event.
type EmitBlob func(key string, data []byte) (objectKey string, err error)

// Collector observes one signal and emits events until the context is cancelled.
type Collector interface {
	// Name is a short identifier used in logs and policy.
	Name() string
	// Start runs the collector; it must return when ctx is cancelled.
	Start(ctx context.Context, emit Emit, emitBlob EmitBlob) error
}
