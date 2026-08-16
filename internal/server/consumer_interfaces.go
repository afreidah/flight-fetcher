// -------------------------------------------------------------------------------
// Server - Consumer-Declared Interfaces
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Narrow read-side views of the data stores the dashboard serves from. The
// server never writes, so every interface here is read-only; splitting them by
// concern rather than by backing store keeps each handler's dependency legible
// and lets a handler degrade independently when its source is unavailable. In
// production these are satisfied by *store.RedisStore, *store.PostgresStore,
// and *hexdb.Client.
// -------------------------------------------------------------------------------

package server

import (
	"context"
	"time"

	"github.com/afreidah/flight-fetcher/internal/aircraft"
	"github.com/afreidah/flight-fetcher/internal/apiclient/opensky"
	"github.com/afreidah/flight-fetcher/internal/route"
	"github.com/afreidah/flight-fetcher/internal/squawk"
)

// flightLister returns current flights from the cache, either the whole set
// for the list view or one aircraft for the detail view. A nil state vector
// with a nil error means the aircraft is not currently being heard.
type flightLister interface {
	GetAllFlights(ctx context.Context) ([]opensky.StateVector, error)
	GetFlight(ctx context.Context, icao24 string) (*opensky.StateVector, error)
}

// aircraftMetaReader retrieves cached aircraft metadata by ICAO24. A sentinel
// record means enrichment ran and found nothing, and callers treat it as
// absent rather than rendering blank fields.
type aircraftMetaReader interface {
	GetAircraftMeta(ctx context.Context, icao24 string) (*aircraft.Info, error)
}

// routeReader retrieves cached flight route information by callsign.
type routeReader interface {
	GetFlightRoute(ctx context.Context, callsign string) (*route.Info, error)
}

// squawkAlertReader retrieves emergency squawk alerts recorded within the
// given lookback window.
type squawkAlertReader interface {
	GetRecentSquawkAlerts(ctx context.Context, since time.Duration) ([]squawk.Alert, error)
}

// imageFetcher resolves an aircraft photo URL by ICAO24, returning an empty
// string when no photo is available. Failures are not an error condition: the
// dashboard simply renders without a photo.
type imageFetcher interface {
	FetchImageURL(ctx context.Context, icao24 string) string
}

// heardChecker reports which sources have observed each aircraft recently.
// The candidate source list is passed in rather than held by the store, so
// the same check works for any set of configured pollers.
type heardChecker interface {
	HeardBy(ctx context.Context, icao24 string, sources []string) ([]string, error)
	HeardByAll(ctx context.Context, icaos, sources []string) (map[string][]string, error)
}

// pinger checks whether a backend dependency is reachable, for /healthz.
type pinger interface {
	Ping(ctx context.Context) error
}
