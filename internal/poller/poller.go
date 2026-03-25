// -------------------------------------------------------------------------------
// Poller - Flight Data Polling Loop
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Runs on a configurable interval, querying the OpenSky Network API for
// aircraft within a bounding box, filtering by haversine distance, storing
// current state in Redis, and logging sightings to Postgres. Enrichment of
// newly seen aircraft is handled asynchronously by a background worker pool.
// -------------------------------------------------------------------------------

package poller

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/afreidah/flight-fetcher/internal/geo"
	"github.com/afreidah/flight-fetcher/internal/opensky"
	"github.com/afreidah/flight-fetcher/internal/runloop"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
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
// CONSTANTS
// -------------------------------------------------------------------------

const (
	enrichWorkers   = 5
	enrichQueueSize = 500
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// enrichRequest is a unit of work for the enrichment worker pool.
type enrichRequest struct {
	icao24   string
	callsign string
}

// Options holds the dependencies and configuration for the poller.
type Options struct {
	Source        FlightSource
	Cache         FlightCache
	Logger        SightingLogger
	Enricher      FlightEnricher
	Center        geo.Coord
	RadiusKm      float64
	Interval      time.Duration
	EvictInterval time.Duration
}

// Poller periodically queries a flight source for aircraft near a fixed location.
type Poller struct {
	opts       Options
	enrichCh   chan enrichRequest

	pollCount    metric.Int64Counter
	pollDuration metric.Float64Histogram
	aircraftGauge metric.Int64Gauge
	enrichQueue  metric.Int64Gauge

	mu         sync.Mutex
	seenICAO   map[string]bool
	seenRoutes map[string]bool
	lastEvict  time.Time
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// New creates a Poller with the given options.
func New(opts *Options) *Poller {
	meter := otel.Meter("flight-fetcher/poller")
	pollCount, _ := meter.Int64Counter("poller.polls",
		metric.WithDescription("Total poll cycles by result"))
	pollDuration, _ := meter.Float64Histogram("poller.poll.duration",
		metric.WithDescription("Poll cycle duration in seconds"),
		metric.WithUnit("s"))
	aircraftGauge, _ := meter.Int64Gauge("poller.aircraft.count",
		metric.WithDescription("Aircraft seen in last poll cycle"))
	enrichQueue, _ := meter.Int64Gauge("poller.enrich.queue",
		metric.WithDescription("Enrichment queue depth"))

	return &Poller{
		opts:          *opts,
		enrichCh:      make(chan enrichRequest, enrichQueueSize),
		pollCount:     pollCount,
		pollDuration:  pollDuration,
		aircraftGauge: aircraftGauge,
		enrichQueue:   enrichQueue,
		seenICAO:      make(map[string]bool),
		seenRoutes:    make(map[string]bool),
		lastEvict:     time.Now(),
	}
}

// Run starts the polling loop and enrichment worker pool. Blocks until
// ctx is cancelled.
func (p *Poller) Run(ctx context.Context) {
	slog.InfoContext(ctx, "poller config",
		slog.Float64("lat", p.opts.Center.Lat),
		slog.Float64("lon", p.opts.Center.Lon),
		slog.Float64("radius_km", p.opts.RadiusKm))

	var wg sync.WaitGroup
	for range enrichWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.enrichWorker(ctx)
		}()
	}

	runloop.Run(ctx, "poller", p.opts.Interval, p.poll)

	close(p.enrichCh)
	wg.Wait()
}

// -------------------------------------------------------------------------
// INTERNALS
// -------------------------------------------------------------------------

// poll executes a single poll cycle: query source, filter by distance,
// store state, log sightings, and submit enrichment requests.
func (p *Poller) poll(ctx context.Context) {
	tracer := otel.Tracer("flight-fetcher/poller")
	ctx, span := tracer.Start(ctx, "poller.poll")
	defer span.End()
	start := time.Now()
	p.mu.Lock()
	if time.Since(p.lastEvict) >= p.opts.EvictInterval {
		slog.InfoContext(ctx, "evicting enrichment cache",
			slog.Int("icao_count", len(p.seenICAO)),
			slog.Int("route_count", len(p.seenRoutes)))
		p.seenICAO = make(map[string]bool)
		p.seenRoutes = make(map[string]bool)
		p.lastEvict = time.Now()
	}
	p.mu.Unlock()

	bbox := geo.BBoxAround(p.opts.Center, p.opts.RadiusKm)
	resp, err := p.opts.Source.GetStates(ctx, bbox)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "poll failed")
		p.pollCount.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "error")))
		p.pollDuration.Record(ctx, time.Since(start).Seconds())
		slog.WarnContext(ctx, "poll failed", slog.String("error", err.Error()))
		return
	}

	count := 0
	for _, sv := range resp.States {
		dist := geo.HaversineKm(p.opts.Center, geo.Coord{Lat: sv.Latitude, Lon: sv.Longitude})
		if dist > p.opts.RadiusKm {
			continue
		}

		if err := p.opts.Cache.SetFlight(ctx, &sv); err != nil {
			slog.WarnContext(ctx, "cache write failed",
				slog.String("icao24", sv.ICAO24),
				slog.String("error", err.Error()))
		}

		if err := p.opts.Logger.LogSighting(ctx, sv.ICAO24, sv.Latitude, sv.Longitude, dist); err != nil {
			slog.WarnContext(ctx, "sighting log failed",
				slog.String("icao24", sv.ICAO24),
				slog.String("error", err.Error()))
		}

		callsign := strings.TrimSpace(sv.Callsign)
		needsEnrich := false

		p.mu.Lock()
		if !p.seenICAO[sv.ICAO24] || (callsign != "" && !p.seenRoutes[callsign]) {
			needsEnrich = true
		}
		p.mu.Unlock()

		if needsEnrich {
			select {
			case p.enrichCh <- enrichRequest{icao24: sv.ICAO24, callsign: callsign}:
			default:
				slog.WarnContext(ctx, "enrichment queue full, skipping",
					slog.String("icao24", sv.ICAO24))
			}
		}
		count++
	}

	span.SetAttributes(attribute.Int("aircraft.count", count))
	p.pollCount.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "ok")))
	p.pollDuration.Record(ctx, time.Since(start).Seconds())
	p.aircraftGauge.Record(ctx, int64(count))
	p.enrichQueue.Record(ctx, int64(len(p.enrichCh)))

	slog.InfoContext(ctx, "poll complete",
		slog.Int("aircraft_count", count),
		slog.Float64("radius_km", p.opts.RadiusKm))
}

// enrichWorker drains the enrichment channel, calling the enricher for
// each request and marking successful enrichments as seen.
func (p *Poller) enrichWorker(ctx context.Context) {
	for req := range p.enrichCh {
		if ctx.Err() != nil {
			return
		}

		p.mu.Lock()
		needICAO := !p.seenICAO[req.icao24]
		needRoute := req.callsign != "" && !p.seenRoutes[req.callsign]
		p.mu.Unlock()

		if needICAO {
			if p.opts.Enricher.Enrich(ctx, req.icao24) {
				p.mu.Lock()
				p.seenICAO[req.icao24] = true
				p.mu.Unlock()
			}
		}
		if needRoute {
			if p.opts.Enricher.EnrichRoute(ctx, req.callsign) {
				p.mu.Lock()
				p.seenRoutes[req.callsign] = true
				p.mu.Unlock()
			}
		}
	}
}
