// -------------------------------------------------------------------------------
// Enricher - Consumer-Declared Interfaces
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Narrow views of the persistence layer required by the enricher, declared
// here rather than exported by the store so the dependency footprint stays
// documented at the point of use. Both are satisfied by *store.PostgresStore;
// neither is exported, because the composition root in cmd/server passes the
// concrete type and no other package constructs an Enricher.
// -------------------------------------------------------------------------------

package enricher

import (
	"context"

	"github.com/afreidah/flight-fetcher/internal/aircraft"
	"github.com/afreidah/flight-fetcher/internal/route"
)

//go:generate mockgen -source consumer_interfaces.go -destination mock_consumer_interfaces_test.go -package enricher

// aircraftMetaReadWriter reads and writes cached aircraft metadata. A nil
// result from the read side means the ICAO24 has never been looked up, which
// is distinct from a cached sentinel recording a confirmed miss.
type aircraftMetaReadWriter interface {
	GetAircraftMeta(ctx context.Context, icao24 string) (*aircraft.Info, error)
	SaveAircraftMeta(ctx context.Context, info *aircraft.Info) error
}

// routeReadWriter reads and writes cached flight route information keyed by
// callsign.
type routeReadWriter interface {
	GetFlightRoute(ctx context.Context, callsign string) (*route.Info, error)
	SaveFlightRoute(ctx context.Context, route *route.Info) error
}
