// -------------------------------------------------------------------------------
// RunLoop - Shared Ticker Loop
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Provides a reusable ticker loop pattern used by the poller, squawk monitor,
// and retention worker. Runs the given function once immediately, then on each
// tick until the context is cancelled.
// -------------------------------------------------------------------------------

package runloop

import (
	"context"
	"log/slog"
	"time"
)

// Run executes fn once immediately, then on each tick of the given interval.
// Blocks until ctx is cancelled. Logs start and stop events using name.
func Run(ctx context.Context, name string, interval time.Duration, fn func(context.Context)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	slog.InfoContext(ctx, name+" started",
		slog.String("interval", interval.String()))

	fn(ctx)
	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, name+" stopped")
			return
		case <-ticker.C:
			fn(ctx)
		}
	}
}
