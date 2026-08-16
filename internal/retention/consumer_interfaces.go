// -------------------------------------------------------------------------------
// Retention - Consumer-Declared Interfaces
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// The narrow view of the persistence layer the cleanup worker needs, declared
// here so the worker's dependency is the three delete calls rather than the
// whole Postgres store. Satisfied by *store.PostgresStore.
// -------------------------------------------------------------------------------

package retention

import (
	"context"
	"time"
)

// cleaner deletes rows older than maxAge from a single table and returns the
// count deleted. Each call deletes at most one batch, so the worker loops
// until a call returns zero to keep lock durations short.
type cleaner interface {
	DeleteOldSightings(ctx context.Context, maxAge time.Duration) (int64, error)
	DeleteOldSquawkAlerts(ctx context.Context, maxAge time.Duration) (int64, error)
	DeleteOldRoutes(ctx context.Context, maxAge time.Duration) (int64, error)
}
