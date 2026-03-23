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
	"time"

	"github.com/afreidah/flight-fetcher/internal/enricher"
	"github.com/afreidah/flight-fetcher/internal/geo"
	"github.com/afreidah/flight-fetcher/internal/opensky"
	"github.com/afreidah/flight-fetcher/internal/store"
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Poller periodically queries OpenSky for aircraft near a fixed location.
type Poller struct {
	opensky  *opensky.Client
	redis    *store.RedisStore
	postgres *store.PostgresStore
	enricher *enricher.Enricher
	center   geo.Coord
	radiusKm float64
	interval time.Duration
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// New creates a Poller with the given dependencies and configuration.
func New(
	osky *opensky.Client,
	redis *store.RedisStore,
	pg *store.PostgresStore,
	enr *enricher.Enricher,
	center geo.Coord,
	radiusKm float64,
	interval time.Duration,
) *Poller {
	return &Poller{
		opensky:  osky,
		redis:    redis,
		postgres: pg,
		enricher: enr,
		center:   center,
		radiusKm: radiusKm,
		interval: interval,
	}
}

// Run starts the polling loop. Blocks until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	slog.InfoContext(ctx, "poller started",
		slog.Float64("lat", p.center.Lat),
		slog.Float64("lon", p.center.Lon),
		slog.Float64("radius_km", p.radiusKm),
		slog.String("interval", p.interval.String()))

	p.poll(ctx)
	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "poller stopped")
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

// -------------------------------------------------------------------------
// INTERNALS
// -------------------------------------------------------------------------

// poll executes a single poll cycle: query OpenSky, filter by distance,
// store state, log sightings, and enrich new aircraft.
func (p *Poller) poll(ctx context.Context) {
	bbox := geo.BBoxAround(p.center, p.radiusKm)
	resp, err := p.opensky.GetStates(ctx, bbox)
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

		if err := p.redis.SetFlight(ctx, sv); err != nil {
			slog.WarnContext(ctx, "redis write failed",
				slog.String("icao24", sv.ICAO24),
				slog.String("error", err.Error()))
		}

		if err := p.postgres.LogSighting(ctx, sv.ICAO24, sv.Latitude, sv.Longitude, dist); err != nil {
			slog.WarnContext(ctx, "sighting log failed",
				slog.String("icao24", sv.ICAO24),
				slog.String("error", err.Error()))
		}

		p.enricher.Enrich(ctx, sv.ICAO24)
		count++
	}

	slog.InfoContext(ctx, "poll complete",
		slog.Int("aircraft_count", count),
		slog.Float64("radius_km", p.radiusKm))
}
