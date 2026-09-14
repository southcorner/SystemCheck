//go:build !windows

package netflow

import (
	"context"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
)

func (c *Collector) run(ctx context.Context, _ collectors.Emit) error {
	<-ctx.Done()
	return nil
}
