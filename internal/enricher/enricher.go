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

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// NamedSource pairs a lookup function with a name for logging.
type NamedSource[T any] struct {
	Name string
	Fn   func(ctx context.Context, key string) (*T, error)
}

// Options holds the dependencies for the enricher. Sources are tried in slice
// order and the first non-nil result wins, so cheaper or more trusted lookups
// belong first. Store and RouteStore are the cache checked before any source
// is called and written back to afterwards.
type Options struct {
	AircraftSources []NamedSource[aircraft.Info]
	Store           aircraftMetaReadWriter
	RouteSources    []NamedSource[route.Info]
	RouteStore      routeReadWriter
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

// enrichResult describes the outcome of an enrichment attempt. ok reports that
// no transient error occurred, so the caller need not retry; found reports that
// data was actually located and saved.
type enrichResult struct {
	ok    bool
	found bool
}

// enrichSpec parameterizes the shared enrichment logic for a given type. label
// and keyLabel name the entity and its key for log messages ("aircraft" and
// "icao24", "route" and "callsign"). get checks the cache, sources are tried in
// order on a miss, and save persists whatever is found. A non-nil sentinel is
// saved when every source comes up empty, recording the confirmed miss so the
// same key is not looked up again. logResult builds the success log attributes.
type enrichSpec[T any] struct {
	label     string
	keyLabel  string
	key       string
	get       func(ctx context.Context, key string) (*T, error)
	sources   []NamedSource[T]
	save      func(ctx context.Context, val *T) error
	sentinel  *T
	logResult func(val *T, source string) slog.Attr
}

// enrich implements the shared check-cache -> try-sources -> save pattern.
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
