// -------------------------------------------------------------------------------
// Enricher - Aircraft Metadata Enrichment
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Orchestrates aircraft metadata enrichment for newly seen ICAO24 codes.
// Checks the Postgres cache first, then queries HexDB.io for unknown aircraft
// and persists the result for future lookups.
// -------------------------------------------------------------------------------

package enricher

import (
	"context"
	"log/slog"

	"github.com/afreidah/flight-fetcher/internal/hexdb"
	"github.com/afreidah/flight-fetcher/internal/store"
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Enricher looks up and caches aircraft metadata from HexDB.io.
type Enricher struct {
	hexdb    *hexdb.Client
	postgres *store.PostgresStore
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// New creates an Enricher backed by the given HexDB client and Postgres store.
func New(h *hexdb.Client, pg *store.PostgresStore) *Enricher {
	return &Enricher{hexdb: h, postgres: pg}
}

// Enrich looks up and caches aircraft metadata if not already known. Returns
// true if this is a newly seen aircraft.
func (e *Enricher) Enrich(ctx context.Context, icao24 string) bool {
	existing, err := e.postgres.GetAircraftMeta(ctx, icao24)
	if err != nil {
		slog.WarnContext(ctx, "failed to check aircraft meta",
			slog.String("icao24", icao24),
			slog.String("error", err.Error()))
		return false
	}
	if existing != nil {
		return false
	}

	info, err := e.hexdb.Lookup(ctx, icao24)
	if err != nil {
		slog.WarnContext(ctx, "hexdb lookup failed",
			slog.String("icao24", icao24),
			slog.String("error", err.Error()))
		return true
	}
	if info == nil {
		slog.InfoContext(ctx, "no hexdb data found",
			slog.String("icao24", icao24))
		return true
	}

	if err := e.postgres.SaveAircraftMeta(ctx, info); err != nil {
		slog.WarnContext(ctx, "failed to save aircraft meta",
			slog.String("icao24", icao24),
			slog.String("error", err.Error()))
	}
	return true
}
