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

// cleanup executes a single cleanup cycle for all tables.
func (w *Worker) cleanup(ctx context.Context) {
	var errs []error

	sightings, err := w.cleaner.DeleteOldSightings(ctx, w.sightingsAge)
	if err != nil {
		errs = append(errs, fmt.Errorf("sightings: %w", err))
	} else if sightings > 0 {
		slog.InfoContext(ctx, "cleaned old sightings",
			slog.Int64("deleted", sightings))
	}

	alerts, err := w.cleaner.DeleteOldSquawkAlerts(ctx, w.alertsAge)
	if err != nil {
		errs = append(errs, fmt.Errorf("squawk alerts: %w", err))
	} else if alerts > 0 {
		slog.InfoContext(ctx, "cleaned old squawk alerts",
			slog.Int64("deleted", alerts))
	}

	routes, err := w.cleaner.DeleteOldRoutes(ctx, w.routesAge)
	if err != nil {
		errs = append(errs, fmt.Errorf("routes: %w", err))
	} else if routes > 0 {
		slog.InfoContext(ctx, "cleaned stale routes",
			slog.Int64("deleted", routes))
	}

	if err := errors.Join(errs...); err != nil {
		slog.WarnContext(ctx, "retention cleanup errors",
			slog.String("error", err.Error()))
	}
}
