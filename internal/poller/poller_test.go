// -------------------------------------------------------------------------------
// Poller - Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests the polling loop logic: radius filtering, error handling for each
// dependency, async enrichment via worker pool, and eviction of seen maps.
// -------------------------------------------------------------------------------

package poller

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/afreidah/flight-fetcher/internal/geo"
	"github.com/afreidah/flight-fetcher/internal/opensky"

	"go.uber.org/mock/gomock"
)

// pollAndDrain runs a single poll cycle and drains the enrichment queue
// by starting workers, then waits for all enrichment to complete.
func pollAndDrain(p *Poller, ctx context.Context) {
	p.poll(ctx)

	var wg sync.WaitGroup
	for range enrichWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.enrichWorker(ctx)
		}()
	}
	close(p.enrichCh)
	wg.Wait()

	// Reset the channel for subsequent calls
	p.enrichCh = make(chan enrichRequest, enrichQueueSize)
}

// TestPoll_FiltersByRadius verifies that only aircraft within the configured radius are processed.
func TestPoll_FiltersByRadius(t *testing.T) {
	ctrl := gomock.NewController(t)
	source := NewMockFlightSource(ctrl)
	cache := NewMockFlightCache(ctrl)
	logger := NewMockSightingLogger(ctrl)
	enricher := NewMockFlightEnricher(ctrl)

	center := geo.Coord{Lat: 34.0928, Lon: -118.3287}
	radiusKm := 10.0

	resp := &opensky.StatesResponse{
		Time: 1234,
		States: []opensky.StateVector{
			{ICAO24: "inside", Callsign: "UAL123", Latitude: 34.09, Longitude: -118.33},
			{ICAO24: "outside", Callsign: "DAL456", Latitude: 35.50, Longitude: -118.33},
		},
	}

	source.EXPECT().
		GetStates(gomock.Any(), gomock.Any()).
		Return(resp, nil)
	cache.EXPECT().
		SetFlight(gomock.Any(), gomock.Any()).
		Return(nil).
		Times(1)
	logger.EXPECT().
		LogSighting(gomock.Any(), "inside", gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).
		Times(1)
	enricher.EXPECT().
		Enrich(gomock.Any(), "inside").
		Return(true).
		Times(1)
	enricher.EXPECT().
		EnrichRoute(gomock.Any(), gomock.Any()).
		Return(true).
		Times(1)

	p := New(&Options{Source: source, Cache: cache, Logger: logger, Enricher: enricher, Center: center, RadiusKm: radiusKm, Interval: time.Minute, EvictInterval: time.Hour})
	pollAndDrain(p, context.Background())
}

// TestPoll_SourceError verifies that a failed API call logs a warning and returns without processing.
func TestPoll_SourceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	source := NewMockFlightSource(ctrl)
	cache := NewMockFlightCache(ctrl)
	logger := NewMockSightingLogger(ctrl)
	enricher := NewMockFlightEnricher(ctrl)

	center := geo.Coord{Lat: 34.0928, Lon: -118.3287}

	source.EXPECT().
		GetStates(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("api down"))

	p := New(&Options{Source: source, Cache: cache, Logger: logger, Enricher: enricher, Center: center, RadiusKm: 50.0, Interval: time.Minute, EvictInterval: time.Hour})
	pollAndDrain(p, context.Background())
}

// TestPoll_CacheError_ContinuesProcessing verifies that a Redis failure does not stop sighting logging or enrichment.
func TestPoll_CacheError_ContinuesProcessing(t *testing.T) {
	ctrl := gomock.NewController(t)
	source := NewMockFlightSource(ctrl)
	cache := NewMockFlightCache(ctrl)
	logger := NewMockSightingLogger(ctrl)
	enricher := NewMockFlightEnricher(ctrl)

	center := geo.Coord{Lat: 34.0928, Lon: -118.3287}

	resp := &opensky.StatesResponse{
		Time: 1234,
		States: []opensky.StateVector{
			{ICAO24: "abc123", Callsign: "AAL100", Latitude: 34.09, Longitude: -118.33},
		},
	}

	source.EXPECT().
		GetStates(gomock.Any(), gomock.Any()).
		Return(resp, nil)
	cache.EXPECT().
		SetFlight(gomock.Any(), gomock.Any()).
		Return(errors.New("redis down"))
	logger.EXPECT().
		LogSighting(gomock.Any(), "abc123", gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	enricher.EXPECT().
		Enrich(gomock.Any(), "abc123").
		Return(true)
	enricher.EXPECT().
		EnrichRoute(gomock.Any(), gomock.Any()).
		Return(true)

	p := New(&Options{Source: source, Cache: cache, Logger: logger, Enricher: enricher, Center: center, RadiusKm: 50.0, Interval: time.Minute, EvictInterval: time.Hour})
	pollAndDrain(p, context.Background())
}

// TestPoll_LoggerError_ContinuesProcessing verifies that a Postgres sighting log failure does not stop enrichment.
func TestPoll_LoggerError_ContinuesProcessing(t *testing.T) {
	ctrl := gomock.NewController(t)
	source := NewMockFlightSource(ctrl)
	cache := NewMockFlightCache(ctrl)
	logger := NewMockSightingLogger(ctrl)
	enricher := NewMockFlightEnricher(ctrl)

	center := geo.Coord{Lat: 34.0928, Lon: -118.3287}

	resp := &opensky.StatesResponse{
		Time: 1234,
		States: []opensky.StateVector{
			{ICAO24: "abc123", Callsign: "AAL100", Latitude: 34.09, Longitude: -118.33},
		},
	}

	source.EXPECT().
		GetStates(gomock.Any(), gomock.Any()).
		Return(resp, nil)
	cache.EXPECT().
		SetFlight(gomock.Any(), gomock.Any()).
		Return(nil)
	logger.EXPECT().
		LogSighting(gomock.Any(), "abc123", gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("pg down"))
	enricher.EXPECT().
		Enrich(gomock.Any(), "abc123").
		Return(true)
	enricher.EXPECT().
		EnrichRoute(gomock.Any(), gomock.Any()).
		Return(true)

	p := New(&Options{Source: source, Cache: cache, Logger: logger, Enricher: enricher, Center: center, RadiusKm: 50.0, Interval: time.Minute, EvictInterval: time.Hour})
	pollAndDrain(p, context.Background())
}

// TestPoll_SkipsEnrichmentOnSecondCycle verifies that already-seen aircraft are not re-enriched.
func TestPoll_SkipsEnrichmentOnSecondCycle(t *testing.T) {
	ctrl := gomock.NewController(t)
	source := NewMockFlightSource(ctrl)
	cache := NewMockFlightCache(ctrl)
	logger := NewMockSightingLogger(ctrl)
	enricher := NewMockFlightEnricher(ctrl)

	center := geo.Coord{Lat: 34.0928, Lon: -118.3287}

	resp := &opensky.StatesResponse{
		Time: 1234,
		States: []opensky.StateVector{
			{ICAO24: "abc123", Callsign: "AAL100", Latitude: 34.09, Longitude: -118.33},
		},
	}

	source.EXPECT().
		GetStates(gomock.Any(), gomock.Any()).
		Return(resp, nil).
		Times(2)
	cache.EXPECT().
		SetFlight(gomock.Any(), gomock.Any()).
		Return(nil).
		Times(2)
	logger.EXPECT().
		LogSighting(gomock.Any(), "abc123", gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).
		Times(2)
	enricher.EXPECT().
		Enrich(gomock.Any(), "abc123").
		Return(true).
		Times(1)
	enricher.EXPECT().
		EnrichRoute(gomock.Any(), "AAL100").
		Return(true).
		Times(1)

	p := New(&Options{Source: source, Cache: cache, Logger: logger, Enricher: enricher, Center: center, RadiusKm: 50.0, Interval: time.Minute, EvictInterval: time.Hour})
	pollAndDrain(p, context.Background())
	// Second poll — enrichment should be skipped since already seen
	pollAndDrain(p, context.Background())
}

// TestPoll_EvictsSeenMapsAfterInterval verifies that seen maps are cleared after the eviction interval.
func TestPoll_EvictsSeenMapsAfterInterval(t *testing.T) {
	ctrl := gomock.NewController(t)
	source := NewMockFlightSource(ctrl)
	cache := NewMockFlightCache(ctrl)
	logger := NewMockSightingLogger(ctrl)
	enricher := NewMockFlightEnricher(ctrl)

	center := geo.Coord{Lat: 34.0928, Lon: -118.3287}

	resp := &opensky.StatesResponse{
		Time: 1234,
		States: []opensky.StateVector{
			{ICAO24: "abc123", Callsign: "AAL100", Latitude: 34.09, Longitude: -118.33},
		},
	}

	// Two poll cycles: first enriches normally, second evicts and re-enriches
	source.EXPECT().
		GetStates(gomock.Any(), gomock.Any()).
		Return(resp, nil).
		Times(2)
	cache.EXPECT().
		SetFlight(gomock.Any(), gomock.Any()).
		Return(nil).
		Times(2)
	logger.EXPECT().
		LogSighting(gomock.Any(), "abc123", gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).
		Times(2)
	// Enrich called on both cycles (eviction clears seen maps before second)
	enricher.EXPECT().
		Enrich(gomock.Any(), "abc123").
		Return(true).
		Times(2)
	enricher.EXPECT().
		EnrichRoute(gomock.Any(), "AAL100").
		Return(true).
		Times(2)

	// Use nanosecond eviction so it triggers on every poll after the first
	p := New(&Options{Source: source, Cache: cache, Logger: logger, Enricher: enricher, Center: center, RadiusKm: 50.0, Interval: time.Minute, EvictInterval: time.Nanosecond})
	pollAndDrain(p, context.Background())
	p.lastEvict = time.Time{} // force eviction on next poll
	pollAndDrain(p, context.Background())
}

// TestPoll_EmptyResponse verifies that an empty states response completes without errors.
func TestPoll_EmptyResponse(t *testing.T) {
	ctrl := gomock.NewController(t)
	source := NewMockFlightSource(ctrl)
	cache := NewMockFlightCache(ctrl)
	logger := NewMockSightingLogger(ctrl)
	enricher := NewMockFlightEnricher(ctrl)

	center := geo.Coord{Lat: 34.0928, Lon: -118.3287}

	source.EXPECT().
		GetStates(gomock.Any(), gomock.Any()).
		Return(&opensky.StatesResponse{Time: 1234, States: nil}, nil)

	p := New(&Options{Source: source, Cache: cache, Logger: logger, Enricher: enricher, Center: center, RadiusKm: 50.0, Interval: time.Minute, EvictInterval: time.Hour})
	pollAndDrain(p, context.Background())
}
