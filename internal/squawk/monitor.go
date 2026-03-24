// -------------------------------------------------------------------------------
// Squawk - Global Emergency Squawk Monitor
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Polls the OpenSky Network API globally on a configurable interval to detect
// aircraft broadcasting emergency squawk codes (7500 hijack, 7600 radio
// failure, 7700 general emergency). Detected alerts are enriched and stored
// in Postgres.
// -------------------------------------------------------------------------------

package squawk

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/afreidah/flight-fetcher/internal/geo"
	"github.com/afreidah/flight-fetcher/internal/opensky"
	"github.com/afreidah/flight-fetcher/internal/runloop"
)

// -------------------------------------------------------------------------
// CONSTANTS
// -------------------------------------------------------------------------

// emergencySquawks are the transponder codes that indicate an emergency.
var emergencySquawks = map[string]bool{
	"7500": true, // hijack
	"7600": true, // radio failure
	"7700": true, // general emergency
}

// -------------------------------------------------------------------------
// INTERFACES
// -------------------------------------------------------------------------

// GlobalFlightSource provides aircraft state vectors without geographic bounds.
type GlobalFlightSource interface {
	GetStates(ctx context.Context, bbox geo.BBox) (*opensky.StatesResponse, error)
}

// AlertStore persists emergency squawk detections.
type AlertStore interface {
	InsertSquawkAlert(ctx context.Context, icao24, callsign, squawk string, lat, lon float64) error
}

// AlertEnricher enriches aircraft metadata and route information.
type AlertEnricher interface {
	Enrich(ctx context.Context, icao24 string) bool
	EnrichRoute(ctx context.Context, callsign string) bool
}

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// alertCooldown is the minimum time between recording duplicate alerts for
// the same aircraft and squawk code.
const alertCooldown = 30 * time.Minute

// Monitor polls for global emergency squawk codes on a configurable interval.
type Monitor struct {
	source   GlobalFlightSource
	store    AlertStore
	enricher AlertEnricher
	interval time.Duration
	seen     map[string]time.Time // key: "icao24:squawk", value: last recorded time
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// New creates a Monitor with the given dependencies and poll interval.
func New(source GlobalFlightSource, store AlertStore, enricher AlertEnricher, interval time.Duration) *Monitor {
	return &Monitor{
		source:   source,
		store:    store,
		enricher: enricher,
		interval: interval,
		seen:     make(map[string]time.Time),
	}
}

// Run starts the monitor loop. Blocks until ctx is cancelled.
func (m *Monitor) Run(ctx context.Context) {
	runloop.Run(ctx, "squawk monitor", m.interval, m.scan)
}

// -------------------------------------------------------------------------
// INTERNALS
// -------------------------------------------------------------------------

// globalBBox returns a bounding box covering the entire world.
func globalBBox() geo.BBox {
	return geo.BBox{MinLat: -90, MaxLat: 90, MinLon: -180, MaxLon: 180}
}

// scan executes a single global poll, filtering for emergency squawk codes.
func (m *Monitor) scan(ctx context.Context) {
	resp, err := m.source.GetStates(ctx, globalBBox())
	if err != nil {
		slog.WarnContext(ctx, "squawk scan failed",
			slog.String("error", err.Error()))
		return
	}

	now := time.Now()
	count := 0
	for _, sv := range resp.States {
		if !emergencySquawks[sv.Squawk] {
			continue
		}

		key := sv.ICAO24 + ":" + sv.Squawk
		if last, ok := m.seen[key]; ok && now.Sub(last) < alertCooldown {
			continue
		}

		callsign := strings.TrimSpace(sv.Callsign)

		slog.WarnContext(ctx, "emergency squawk detected",
			slog.String("icao24", sv.ICAO24),
			slog.String("callsign", callsign),
			slog.String("squawk", sv.Squawk),
			slog.Float64("lat", sv.Latitude),
			slog.Float64("lon", sv.Longitude))

		if err := m.store.InsertSquawkAlert(ctx, sv.ICAO24, callsign, sv.Squawk, sv.Latitude, sv.Longitude); err != nil {
			slog.WarnContext(ctx, "failed to store squawk alert",
				slog.String("icao24", sv.ICAO24),
				slog.String("error", err.Error()))
		}

		m.seen[key] = now

		m.enricher.Enrich(ctx, sv.ICAO24)
		if callsign != "" {
			m.enricher.EnrichRoute(ctx, callsign)
		}

		count++
	}

	// Evict expired entries
	for key, last := range m.seen {
		if now.Sub(last) >= alertCooldown {
			delete(m.seen, key)
		}
	}

	slog.InfoContext(ctx, "squawk scan complete",
		slog.Int("total_aircraft", len(resp.States)),
		slog.Int("emergency_count", count))
}
