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
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/afreidah/flight-fetcher/internal/enricher"
	"github.com/afreidah/flight-fetcher/internal/geo"
	"github.com/afreidah/flight-fetcher/internal/notify"
	"github.com/afreidah/flight-fetcher/internal/apiclient/opensky"
	"github.com/afreidah/flight-fetcher/internal/runloop"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// -------------------------------------------------------------------------
// CONSTANTS
// -------------------------------------------------------------------------

const (
	SquawkHijack       = "7500" // aircraft hijacking
	SquawkRadioFailure = "7600" // radio/communications failure
	SquawkEmergency    = "7700" // general emergency
)

// isEmergencySquawk returns true if the code is an emergency transponder code.
func isEmergencySquawk(code string) bool {
	switch code {
	case SquawkHijack, SquawkRadioFailure, SquawkEmergency:
		return true
	}
	return false
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
	HasRecentSquawkAlert(ctx context.Context, icao24, squawk string, cooldown time.Duration) (bool, error)
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
	enricher enricher.Interface
	notifier notify.Notifier
	interval time.Duration
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// New creates a Monitor with the given dependencies and poll interval.
// The notifier receives alerts for all detected emergencies; use a
// notify.Manager to fan out to multiple backends.
func New(source GlobalFlightSource, store AlertStore, enr enricher.Interface, notifier notify.Notifier, interval time.Duration) *Monitor {
	return &Monitor{
		source:   source,
		store:    store,
		enricher: enr,
		notifier: notifier,
		interval: interval,
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
	tracer := otel.Tracer("flight-fetcher/squawk")
	ctx, span := tracer.Start(ctx, "squawk.scan")
	defer span.End()

	resp, err := m.source.GetStates(ctx, globalBBox())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "squawk scan failed")
		slog.WarnContext(ctx, "squawk scan failed",
			slog.String("error", err.Error()))
		return
	}

	count := 0
	for _, sv := range resp.States {
		if !isEmergencySquawk(sv.Squawk) {
			continue
		}

		exists, err := m.store.HasRecentSquawkAlert(ctx, sv.ICAO24, sv.Squawk, alertCooldown)
		if err != nil {
			slog.WarnContext(ctx, "failed to check recent squawk alert",
				slog.String("icao24", sv.ICAO24),
				slog.String("error", err.Error()))
			continue
		}
		if exists {
			continue
		}

		callsign := strings.TrimSpace(sv.Callsign)

		slog.WarnContext(ctx, "emergency squawk detected",
			slog.Group("alert",
				slog.String("icao24", sv.ICAO24),
				slog.String("callsign", callsign),
				slog.String("squawk", sv.Squawk),
				slog.Float64("lat", sv.Latitude),
				slog.Float64("lon", sv.Longitude)))

		if err := m.store.InsertSquawkAlert(ctx, sv.ICAO24, callsign, sv.Squawk, sv.Latitude, sv.Longitude); err != nil {
			slog.WarnContext(ctx, "failed to store squawk alert",
				slog.String("icao24", sv.ICAO24),
				slog.String("error", err.Error()))
		}

		m.sendNotifications(ctx, sv.ICAO24, callsign, sv.Squawk, sv.Latitude, sv.Longitude)

		m.enricher.Enrich(ctx, sv.ICAO24)
		if callsign != "" {
			m.enricher.EnrichRoute(ctx, callsign)
		}

		count++
	}

	span.SetAttributes(
		attribute.Int("total_aircraft", len(resp.States)),
		attribute.Int("emergency_count", count))

	slog.InfoContext(ctx, "squawk scan complete",
		slog.Int("total_aircraft", len(resp.States)),
		slog.Int("emergency_count", count))
}

// sendNotifications sends an alert via the configured notifier. Errors are
// logged but do not interrupt the scan loop.
func (m *Monitor) sendNotifications(ctx context.Context, icao24, callsign, code string, lat, lon float64) {
	if m.notifier == nil {
		return
	}

	msg := notify.Message{
		Title: "Emergency Squawk: " + squawkLabel(code),
		Body:  fmt.Sprintf("Aircraft %s broadcasting squawk %s", icao24, code),
		Fields: []notify.Field{
			{Name: "ICAO24", Value: icao24},
			{Name: "Callsign", Value: callsign},
			{Name: "Squawk", Value: code + " (" + squawkLabel(code) + ")"},
			{Name: "Position", Value: fmt.Sprintf("%.4f, %.4f", lat, lon)},
		},
	}

	if err := m.notifier.Send(ctx, msg); err != nil {
		slog.WarnContext(ctx, "failed to send notification",
			slog.String("icao24", icao24),
			slog.String("error", err.Error()))
	}
}

// squawkLabel returns a human-readable description for a squawk code.
func squawkLabel(code string) string {
	switch code {
	case SquawkHijack:
		return "Hijack"
	case SquawkRadioFailure:
		return "Radio Failure"
	case SquawkEmergency:
		return "General Emergency"
	default:
		return "Unknown"
	}
}
