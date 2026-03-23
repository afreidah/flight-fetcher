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
)

//go:generate mockgen -destination mock_enricher_test.go -package enricher github.com/afreidah/flight-fetcher/internal/enricher AircraftStore,AircraftLookup

// -------------------------------------------------------------------------
// INTERFACES
// -------------------------------------------------------------------------

// AircraftStore reads and writes cached aircraft metadata.
type AircraftStore interface {
	GetAircraftMeta(ctx context.Context, icao24 string) (*hexdb.AircraftInfo, error)
	SaveAircraftMeta(ctx context.Context, info *hexdb.AircraftInfo) error
}

// AircraftLookup fetches aircraft metadata from an external source.
type AircraftLookup interface {
	Lookup(ctx context.Context, icao24 string) (*hexdb.AircraftInfo, error)
}

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Enricher looks up and caches aircraft metadata from an external source.
type Enricher struct {
	lookup AircraftLookup
	store  AircraftStore
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// New creates an Enricher backed by the given lookup client and metadata store.
func New(lookup AircraftLookup, store AircraftStore) *Enricher {
	return &Enricher{lookup: lookup, store: store}
}

// Enrich looks up and caches aircraft metadata if not already known. Returns
// true if this is a newly seen aircraft.
func (e *Enricher) Enrich(ctx context.Context, icao24 string) bool {
	existing, err := e.store.GetAircraftMeta(ctx, icao24)
	if err != nil {
		slog.WarnContext(ctx, "failed to check aircraft meta",
			slog.String("icao24", icao24),
			slog.String("error", err.Error()))
		return false
	}
	if existing != nil {
		return false
	}

	info, err := e.lookup.Lookup(ctx, icao24)
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

	if err := e.store.SaveAircraftMeta(ctx, info); err != nil {
		slog.WarnContext(ctx, "failed to save aircraft meta",
			slog.String("icao24", icao24),
			slog.String("error", err.Error()))
	}
	return true
}
