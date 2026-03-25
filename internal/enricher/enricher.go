// -------------------------------------------------------------------------------
// Enricher - Aircraft Metadata and Route Enrichment
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Orchestrates aircraft metadata enrichment for newly seen ICAO24 codes and
// flight route enrichment for newly seen callsigns. Checks the Postgres cache
// first, then queries external APIs (HexDB.io for metadata, AirLabs for
// routes) and persists results for future lookups.
// -------------------------------------------------------------------------------

package enricher

import (
	"context"
	"log/slog"

	"github.com/afreidah/flight-fetcher/internal/aircraft"
	"github.com/afreidah/flight-fetcher/internal/route"
)

//go:generate mockgen -destination mock_enricher_test.go -package enricher github.com/afreidah/flight-fetcher/internal/enricher AircraftStore,AircraftLookup,RouteStore,RouteLookup

// -------------------------------------------------------------------------
// INTERFACES
// -------------------------------------------------------------------------

// AircraftStore reads and writes cached aircraft metadata.
type AircraftStore interface {
	GetAircraftMeta(ctx context.Context, icao24 string) (*aircraft.Info, error)
	SaveAircraftMeta(ctx context.Context, info *aircraft.Info) error
}

// AircraftLookup fetches aircraft metadata from an external source.
type AircraftLookup interface {
	Lookup(ctx context.Context, icao24 string) (*aircraft.Info, error)
}

// RouteStore reads and writes cached flight route information.
type RouteStore interface {
	GetFlightRoute(ctx context.Context, callsign string) (*route.Info, error)
	SaveFlightRoute(ctx context.Context, route *route.Info) error
}

// RouteLookup fetches flight route information from an external source.
type RouteLookup interface {
	LookupRoute(ctx context.Context, callsign string) (*route.Info, error)
}

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Options holds the dependencies for the enricher.
type Options struct {
	Lookup        AircraftLookup
	Store         AircraftStore
	RouteLookup   RouteLookup
	RouteFallback RouteLookup
	RouteStore    RouteStore
}

// Enricher looks up and caches aircraft metadata and flight route information.
type Enricher struct {
	opts Options
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// New creates an Enricher with the given options. Route enrichment is enabled
// when RouteLookup and RouteStore are non-nil.
func New(opts *Options) *Enricher {
	return &Enricher{opts: *opts}
}

// Enrich looks up and caches aircraft metadata if not already known. Returns
// true when enrichment is complete (data cached or confirmed absent). Returns
// false on transient errors so the caller can retry.
func (e *Enricher) Enrich(ctx context.Context, icao24 string) bool {
	existing, err := e.opts.Store.GetAircraftMeta(ctx, icao24)
	if err != nil {
		slog.WarnContext(ctx, "failed to check aircraft meta",
			slog.String("icao24", icao24),
			slog.String("error", err.Error()))
		return false
	}
	if existing != nil {
		return true
	}

	slog.InfoContext(ctx, "enriching aircraft",
		slog.String("icao24", icao24))

	info, err := e.opts.Lookup.Lookup(ctx, icao24)
	if err != nil {
		slog.WarnContext(ctx, "hexdb lookup failed",
			slog.String("icao24", icao24),
			slog.String("error", err.Error()))
		return false
	}
	if info == nil {
		slog.InfoContext(ctx, "no hexdb data found",
			slog.String("icao24", icao24))
		sentinel := &aircraft.Info{ICAO24: icao24}
		if err := e.opts.Store.SaveAircraftMeta(ctx, sentinel); err != nil {
			slog.WarnContext(ctx, "failed to save aircraft sentinel",
				slog.String("icao24", icao24),
				slog.String("error", err.Error()))
		}
		return true
	}

	slog.InfoContext(ctx, "aircraft enriched",
		slog.Group("aircraft",
			slog.String("icao24", icao24),
			slog.String("registration", info.Registration),
			slog.String("type", info.Type)))

	if err := e.opts.Store.SaveAircraftMeta(ctx, info); err != nil {
		slog.WarnContext(ctx, "failed to save aircraft meta",
			slog.String("icao24", icao24),
			slog.String("error", err.Error()))
	}
	return true
}

// EnrichRoute looks up and caches flight route information if not already
// known. Returns true when complete (cached or confirmed absent), false on
// transient errors so the caller can retry. No-op (returns true) when route
// enrichment is not configured.
func (e *Enricher) EnrichRoute(ctx context.Context, callsign string) bool {
	if e.opts.RouteLookup == nil || e.opts.RouteStore == nil {
		return true
	}

	existing, err := e.opts.RouteStore.GetFlightRoute(ctx, callsign)
	if err != nil {
		slog.WarnContext(ctx, "failed to check flight route",
			slog.String("callsign", callsign),
			slog.String("error", err.Error()))
		return false
	}
	if existing != nil {
		return true
	}

	slog.InfoContext(ctx, "enriching route",
		slog.String("callsign", callsign))

	route, err := e.opts.RouteLookup.LookupRoute(ctx, callsign)
	if err != nil {
		slog.WarnContext(ctx, "primary route lookup failed",
			slog.String("callsign", callsign),
			slog.String("error", err.Error()))
		if e.opts.RouteFallback != nil {
			route, err = e.opts.RouteFallback.LookupRoute(ctx, callsign)
			if err != nil {
				slog.WarnContext(ctx, "fallback route lookup failed",
					slog.String("callsign", callsign),
					slog.String("error", err.Error()))
				return false
			}
		} else {
			return false
		}
	}
	if route == nil {
		slog.InfoContext(ctx, "no route data found",
			slog.String("callsign", callsign))
		return true
	}

	slog.InfoContext(ctx, "route enriched",
		slog.Group("route",
			slog.String("callsign", callsign),
			slog.String("from", route.DepIATA),
			slog.String("to", route.ArrIATA)))

	if err := e.opts.RouteStore.SaveFlightRoute(ctx, route); err != nil {
		slog.WarnContext(ctx, "failed to save flight route",
			slog.String("callsign", callsign),
			slog.String("error", err.Error()))
	}
	return true
}
