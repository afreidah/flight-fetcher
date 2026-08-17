// -------------------------------------------------------------------------------
// Poller - Consumer-Declared Interfaces
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Narrow views of the collaborators the polling loop needs, declared here so
// each dependency is documented as the handful of methods actually called
// rather than the full surface of the type that supplies them. In production
// these are satisfied by the OpenSky and dump1090 clients, *store.RedisStore,
// *store.PostgresStore, and *enricher.Enricher.
// -------------------------------------------------------------------------------

package poller

import (
	"context"
	"time"

	"github.com/afreidah/flight-fetcher/internal/apiclient/opensky"
	"github.com/afreidah/flight-fetcher/internal/geo"
)

//go:generate mockgen -source consumer_interfaces.go -destination mock_consumer_interfaces_test.go -package poller

// stateFetcher provides aircraft state vectors for a geographic area. The
// bounding box is a coarse pre-filter; the poller still applies an exact
// haversine radius check to the results.
type stateFetcher interface {
	GetStates(ctx context.Context, bbox geo.BBox) (*opensky.StatesResponse, error)
}

// flightCache stores current flight state for fast lookup and records
// per-source liveness so readers can tell which source is currently
// hearing each aircraft.
type flightCache interface {
	SetFlight(ctx context.Context, sv *opensky.StateVector) error
	MarkHeard(ctx context.Context, source, icao24 string, ttl time.Duration) error
}

// sightingLogger records historical aircraft sightings.
type sightingLogger interface {
	LogSighting(ctx context.Context, icao24 string, lat, lon, distanceKm float64) error
}

// aircraftEnricher resolves metadata and routes for aircraft the pipeline has
// not seen before. Both methods report ok=false only on transient failure, so
// the poller can distinguish "retry later" from "confirmed absent" and avoid
// re-queueing keys that will never resolve.
type aircraftEnricher interface {
	Enrich(ctx context.Context, icao24 string) bool
	EnrichRoute(ctx context.Context, callsign string) (ok bool, found bool)
}
