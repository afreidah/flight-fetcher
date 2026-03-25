// -------------------------------------------------------------------------------
// Retention - Data Cleanup Worker
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Periodically deletes old sightings and squawk alerts from Postgres to
// prevent unbounded table growth. Runs on a configurable interval with
// configurable max ages per table.
// -------------------------------------------------------------------------------

package retention

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/afreidah/flight-fetcher/internal/runloop"
)

// -------------------------------------------------------------------------
// INTERFACES
// -------------------------------------------------------------------------

// Cleaner deletes old rows from a table and returns the count deleted.
type Cleaner interface {
	DeleteOldSightings(ctx context.Context, maxAge time.Duration) (int64, error)
	DeleteOldSquawkAlerts(ctx context.Context, maxAge time.Duration) (int64, error)
	DeleteOldRoutes(ctx context.Context, maxAge time.Duration) (int64, error)
}

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Worker periodically cleans up old data from Postgres.
type Worker struct {
	cleaner      Cleaner
	sightingsAge time.Duration
	alertsAge    time.Duration
	routesAge    time.Duration
	interval     time.Duration
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// New creates a retention Worker with the given cleanup parameters.
func New(cleaner Cleaner, sightingsAge, alertsAge, routesAge, interval time.Duration) *Worker {
	return &Worker{
		cleaner:      cleaner,
		sightingsAge: sightingsAge,
		alertsAge:    alertsAge,
		routesAge:    routesAge,
		interval:     interval,
	}
}

// Run starts the cleanup loop. Blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	slog.InfoContext(ctx, "retention worker config",
		slog.String("sightings_max_age", w.sightingsAge.String()),
		slog.String("alerts_max_age", w.alertsAge.String()),
		slog.String("routes_max_age", w.routesAge.String()))
	runloop.Run(ctx, "retention worker", w.interval, w.cleanup)
}

// -------------------------------------------------------------------------
// INTERNALS
// -------------------------------------------------------------------------

// cleanup executes a single cleanup cycle for all tables. Each delete is
// batched (up to 10000 rows per call) and looped until no rows remain,
// keeping lock durations short.
func (w *Worker) cleanup(ctx context.Context) {
	var errs []error

	if err := w.deleteInBatches(ctx, "sightings", func(ctx context.Context) (int64, error) {
		return w.cleaner.DeleteOldSightings(ctx, w.sightingsAge)
	}); err != nil {
		errs = append(errs, fmt.Errorf("sightings: %w", err))
	}

	if err := w.deleteInBatches(ctx, "squawk alerts", func(ctx context.Context) (int64, error) {
		return w.cleaner.DeleteOldSquawkAlerts(ctx, w.alertsAge)
	}); err != nil {
		errs = append(errs, fmt.Errorf("squawk alerts: %w", err))
	}

	if err := w.deleteInBatches(ctx, "routes", func(ctx context.Context) (int64, error) {
		return w.cleaner.DeleteOldRoutes(ctx, w.routesAge)
	}); err != nil {
		errs = append(errs, fmt.Errorf("routes: %w", err))
	}

	if err := errors.Join(errs...); err != nil {
		slog.WarnContext(ctx, "retention cleanup errors",
			slog.String("error", err.Error()))
	}
}

// deleteInBatches calls deleteFn repeatedly until it returns 0 rows,
// logging the total rows deleted.
func (w *Worker) deleteInBatches(ctx context.Context, name string, deleteFn func(context.Context) (int64, error)) error {
	var total int64
	for {
		n, err := deleteFn(ctx)
		if err != nil {
			return err
		}
		total += n
		if n == 0 {
			break
		}
	}
	if total > 0 {
		slog.InfoContext(ctx, "cleaned old "+name,
			slog.Int64("deleted", total))
	}
	return nil
}
