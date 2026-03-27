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

//go:generate mockgen -destination mock_enricher_test.go -package enricher github.com/afreidah/flight-fetcher/internal/enricher AircraftStore,RouteStore

// -------------------------------------------------------------------------
// INTERFACES
// -------------------------------------------------------------------------

// AircraftStore reads and writes cached aircraft metadata.
type AircraftStore interface {
	GetAircraftMeta(ctx context.Context, icao24 string) (*aircraft.Info, error)
	SaveAircraftMeta(ctx context.Context, info *aircraft.Info) error
}

// RouteStore reads and writes cached flight route information.
type RouteStore interface {
	GetFlightRoute(ctx context.Context, callsign string) (*route.Info, error)
	SaveFlightRoute(ctx context.Context, route *route.Info) error
}

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// NamedSource pairs a lookup function with a name for logging.
type NamedSource[T any] struct {
	Name string
	Fn   func(ctx context.Context, key string) (*T, error)
}

// Options holds the dependencies for the enricher.
type Options struct {
	AircraftSources []NamedSource[aircraft.Info]
	Store           AircraftStore
	RouteSources    []NamedSource[route.Info]
	RouteStore      RouteStore
}

// Interface is satisfied by Enricher and describes the enrichment
// operations consumed by the poller and squawk monitor.
type Interface interface {
	Enrich(ctx context.Context, icao24 string) bool
	EnrichRoute(ctx context.Context, callsign string) (ok bool, found bool)
}

// Enricher looks up and caches aircraft metadata and flight route information.
type Enricher struct {
	opts Options
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// New creates an Enricher with the given options. Route enrichment is enabled
// when RouteSources and RouteStore are non-nil.
func New(opts *Options) *Enricher {
	return &Enricher{opts: *opts}
}

// Enrich looks up and caches aircraft metadata if not already known. Returns
// true when enrichment is complete (data cached or confirmed absent). Returns
// false on transient errors so the caller can retry.
func (e *Enricher) Enrich(ctx context.Context, icao24 string) bool {
	r := enrich(ctx, enrichSpec[aircraft.Info]{
		label:    "aircraft",
		keyLabel: "icao24",
		key:      icao24,
		get:      e.opts.Store.GetAircraftMeta,
		sources:  e.opts.AircraftSources,
		save:     e.opts.Store.SaveAircraftMeta,
		sentinel: &aircraft.Info{ICAO24: icao24},
		logResult: func(info *aircraft.Info, source string) slog.Attr {
			return slog.Group("aircraft",
				slog.String("icao24", icao24),
				slog.String("registration", info.Registration),
				slog.String("type", info.Type),
				slog.String("source", source))
		},
	})
	return r.ok
}

// EnrichRoute looks up and caches flight route information if not already
// known. Returns (ok, found) where ok means no transient error and found
// means route data was actually located and saved. No-op (true, true) when
// route enrichment is not configured.
func (e *Enricher) EnrichRoute(ctx context.Context, callsign string) (bool, bool) {
	if len(e.opts.RouteSources) == 0 || e.opts.RouteStore == nil {
		return true, true
	}
	r := enrich(ctx, enrichSpec[route.Info]{
		label:    "route",
		keyLabel: "callsign",
		key:      callsign,
		get:      e.opts.RouteStore.GetFlightRoute,
		sources:  e.opts.RouteSources,
		save:     e.opts.RouteStore.SaveFlightRoute,
		logResult: func(ri *route.Info, source string) slog.Attr {
			return slog.Group("route",
				slog.String("callsign", callsign),
				slog.String("from", ri.DepIATA),
				slog.String("to", ri.ArrIATA),
				slog.String("source", source))
		},
	})
	return r.ok, r.found
}

// -------------------------------------------------------------------------
// INTERNALS
// -------------------------------------------------------------------------

// enrichResult describes the outcome of an enrichment attempt.
type enrichResult struct {
	ok    bool // true if complete (no transient error)
	found bool // true if data was found and saved
}

// enrichSpec parameterizes the shared enrichment logic for a given type.
type enrichSpec[T any] struct {
	label     string                                            // e.g. "aircraft", "route"
	keyLabel  string                                            // e.g. "icao24", "callsign"
	key       string                                            // the lookup key
	get       func(ctx context.Context, key string) (*T, error) // cache check
	sources   []NamedSource[T]                                  // ordered lookup sources
	save      func(ctx context.Context, val *T) error           // persist result
	sentinel  *T                                                // if non-nil, saved on miss
	logResult func(val *T, source string) slog.Attr             // success log attributes
}

// enrich implements the shared check-cache → try-sources → save pattern.
func enrich[T any](ctx context.Context, spec enrichSpec[T]) enrichResult {
	existing, err := spec.get(ctx, spec.key)
	if err != nil {
		slog.WarnContext(ctx, "failed to check "+spec.label,
			slog.String(spec.keyLabel, spec.key),
			slog.String("error", err.Error()))
		return enrichResult{ok: false}
	}
	if existing != nil {
		return enrichResult{ok: true, found: true}
	}

	var result *T
	var triedSources []string
	for _, src := range spec.sources {
		triedSources = append(triedSources, src.Name)
		slog.InfoContext(ctx, "enriching "+spec.label,
			slog.String(spec.keyLabel, spec.key),
			slog.String("source", src.Name))

		val, err := src.Fn(ctx, spec.key)
		if err != nil {
			slog.WarnContext(ctx, spec.label+" lookup failed",
				slog.String(spec.keyLabel, spec.key),
				slog.String("source", src.Name),
				slog.String("error", err.Error()))
			continue
		}
		if val != nil {
			result = val
			slog.InfoContext(ctx, spec.label+" enriched", spec.logResult(val, src.Name))
			break
		}
	}

	if result == nil {
		slog.InfoContext(ctx, "no "+spec.label+" data found",
			slog.String(spec.keyLabel, spec.key),
			slog.Any("sources_tried", triedSources))
		if spec.sentinel != nil {
			if err := spec.save(ctx, spec.sentinel); err != nil {
				slog.WarnContext(ctx, "failed to save "+spec.label+" sentinel",
					slog.String(spec.keyLabel, spec.key),
					slog.String("error", err.Error()))
			}
		}
		return enrichResult{ok: true, found: false}
	}

	if err := spec.save(ctx, result); err != nil {
		slog.WarnContext(ctx, "failed to save "+spec.label,
			slog.String(spec.keyLabel, spec.key),
			slog.String("error", err.Error()))
	}
	return enrichResult{ok: true, found: true}
}
