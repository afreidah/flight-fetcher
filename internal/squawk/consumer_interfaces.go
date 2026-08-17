// -------------------------------------------------------------------------------
// Squawk - Consumer-Declared Interfaces
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Narrow views of the collaborators the emergency squawk monitor needs. The
// enricher view is declared here rather than shared with the poller: both
// consume the same two methods today, but keeping the declaration local means
// a method added for one consumer does not widen the other's mock.
// -------------------------------------------------------------------------------

package squawk

import (
	"context"
	"time"

	"github.com/afreidah/flight-fetcher/internal/apiclient/opensky"
	"github.com/afreidah/flight-fetcher/internal/geo"
)

// globalStateFetcher provides aircraft state vectors without geographic
// bounds. The monitor passes a world-covering bounding box, so any source
// that honours the box supplies the global feed.
type globalStateFetcher interface {
	GetStates(ctx context.Context, bbox geo.BBox) (*opensky.StatesResponse, error)
}

// alertRecorder persists emergency squawk detections and reports whether an
// equivalent detection was already recorded inside the cooldown window, which
// is what keeps a continuously broadcasting aircraft from generating an alert
// on every scan.
type alertRecorder interface {
	InsertSquawkAlert(ctx context.Context, icao24, callsign, squawk string, lat, lon float64) error
	HasRecentSquawkAlert(ctx context.Context, icao24, squawk string, cooldown time.Duration) (bool, error)
}

// aircraftEnricher resolves metadata and routes for aircraft named in an
// alert. The monitor calls it on a background goroutine and ignores the
// result, since enrichment is a display convenience and must never delay
// detection.
type aircraftEnricher interface {
	Enrich(ctx context.Context, icao24 string) bool
	EnrichRoute(ctx context.Context, callsign string) (ok bool, found bool)
}
