// -------------------------------------------------------------------------------
// Poller - Flight Data Polling Loop
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Runs on a configurable interval, querying the OpenSky Network API for
// aircraft within a bounding box, filtering by haversine distance, storing
// current state in Redis, logging sightings to Postgres, and triggering
// enrichment for newly seen aircraft.
// -------------------------------------------------------------------------------

package poller

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/afreidah/flight-fetcher/internal/geo"
	"github.com/afreidah/flight-fetcher/internal/opensky"
	"github.com/afreidah/flight-fetcher/internal/runloop"
)

//go:generate mockgen -destination mock_poller_test.go -package poller github.com/afreidah/flight-fetcher/internal/poller FlightSource,FlightCache,SightingLogger,FlightEnricher

// -------------------------------------------------------------------------
// INTERFACES
// -------------------------------------------------------------------------

// FlightSource provides aircraft state vectors for a geographic area.
type FlightSource interface {
	GetStates(ctx context.Context, bbox geo.BBox) (*opensky.StatesResponse, error)
}

// FlightCache stores current flight state for fast lookup.
type FlightCache interface {
	SetFlight(ctx context.Context, sv *opensky.StateVector) error
}

// SightingLogger records historical aircraft sightings.
type SightingLogger interface {
	LogSighting(ctx context.Context, icao24 string, lat, lon, distanceKm float64) error
}

// FlightEnricher enriches aircraft metadata and flight route information.
type FlightEnricher interface {
	Enrich(ctx context.Context, icao24 string) bool
	EnrichRoute(ctx context.Context, callsign string) bool
}

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Poller periodically queries a flight source for aircraft near a fixed location.
type Poller struct {
	source        FlightSource
	cache         FlightCache
	logger        SightingLogger
	enricher      FlightEnricher
	center        geo.Coord
	radiusKm      float64
	interval      time.Duration
	seenICAO      map[string]bool
	seenRoutes    map[string]bool
	evictInterval time.Duration
	lastEvict     time.Time
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// New creates a Poller with the given dependencies and configuration.
func New(
	source FlightSource,
	cache FlightCache,
	logger SightingLogger,
	enr FlightEnricher,
	center geo.Coord,
	radiusKm float64,
	interval time.Duration,
	evictInterval time.Duration,
) *Poller {
	return &Poller{
		source:        source,
		cache:         cache,
		logger:        logger,
		enricher:      enr,
		center:        center,
		radiusKm:      radiusKm,
		interval:      interval,
		seenICAO:      make(map[string]bool),
		seenRoutes:    make(map[string]bool),
		evictInterval: evictInterval,
		lastEvict:     time.Now(),
	}
}

// Run starts the polling loop. Blocks until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) {
	slog.InfoContext(ctx, "poller config",
		slog.Float64("lat", p.center.Lat),
		slog.Float64("lon", p.center.Lon),
		slog.Float64("radius_km", p.radiusKm))
	runloop.Run(ctx, "poller", p.interval, p.poll)
}

// -------------------------------------------------------------------------
// INTERNALS
// -------------------------------------------------------------------------

// poll executes a single poll cycle: query source, filter by distance,
// store state, log sightings, and enrich new aircraft.
func (p *Poller) poll(ctx context.Context) {
	if time.Since(p.lastEvict) >= p.evictInterval {
		slog.InfoContext(ctx, "evicting enrichment cache",
			slog.Int("icao_count", len(p.seenICAO)),
			slog.Int("route_count", len(p.seenRoutes)))
		p.seenICAO = make(map[string]bool)
		p.seenRoutes = make(map[string]bool)
		p.lastEvict = time.Now()
	}

	bbox := geo.BBoxAround(p.center, p.radiusKm)
	resp, err := p.source.GetStates(ctx, bbox)
	if err != nil {
		slog.WarnContext(ctx, "poll failed", slog.String("error", err.Error()))
		return
	}

	count := 0
	for _, sv := range resp.States {
		dist := geo.HaversineKm(p.center, geo.Coord{Lat: sv.Latitude, Lon: sv.Longitude})
		if dist > p.radiusKm {
			continue
		}

		if err := p.cache.SetFlight(ctx, &sv); err != nil {
			slog.WarnContext(ctx, "cache write failed",
				slog.String("icao24", sv.ICAO24),
				slog.String("error", err.Error()))
		}

		if err := p.logger.LogSighting(ctx, sv.ICAO24, sv.Latitude, sv.Longitude, dist); err != nil {
			slog.WarnContext(ctx, "sighting log failed",
				slog.String("icao24", sv.ICAO24),
				slog.String("error", err.Error()))
		}

		if !p.seenICAO[sv.ICAO24] {
			if p.enricher.Enrich(ctx, sv.ICAO24) {
				p.seenICAO[sv.ICAO24] = true
			}
		}
		if callsign := strings.TrimSpace(sv.Callsign); callsign != "" && !p.seenRoutes[callsign] {
			if p.enricher.EnrichRoute(ctx, callsign) {
				p.seenRoutes[callsign] = true
			}
		}
		count++
	}

	slog.InfoContext(ctx, "poll complete",
		slog.Int("aircraft_count", count),
		slog.Float64("radius_km", p.radiusKm))
}
