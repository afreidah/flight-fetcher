// -------------------------------------------------------------------------------
// RunLoop - Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests the shared ticker loop: immediate execution, repeated ticks, and
// clean shutdown on context cancellation.
// -------------------------------------------------------------------------------

package runloop

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestRun_ExecutesImmediately verifies that fn is called once before the first tick.
func TestRun_ExecutesImmediately(t *testing.T) {
	var count atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())

	go Run(ctx, "test", time.Hour, func(_ context.Context) {
		count.Add(1)
		cancel()
	})

	<-ctx.Done()
	if got := count.Load(); got != 1 {
		t.Errorf("count = %d, want 1", got)
	}
}

// TestRun_TicksRepeatedly verifies that fn is called on each tick.
func TestRun_TicksRepeatedly(t *testing.T) {
	var count atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())

	go Run(ctx, "test", 10*time.Millisecond, func(_ context.Context) {
		if count.Add(1) >= 3 {
			cancel()
		}
	})

	<-ctx.Done()
	if got := count.Load(); got < 3 {
		t.Errorf("count = %d, want >= 3", got)
	}
}

// TestRun_StopsOnCancel verifies that Run returns when the context is cancelled.
func TestRun_StopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		Run(ctx, "test", time.Hour, func(_ context.Context) {})
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}
