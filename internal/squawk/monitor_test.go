// -------------------------------------------------------------------------------
// Squawk - Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests the global squawk monitor: emergency detection, non-emergency filtering,
// empty responses, source errors, and enrichment of detected aircraft.
// -------------------------------------------------------------------------------

package squawk

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/afreidah/flight-fetcher/internal/geo"
	"github.com/afreidah/flight-fetcher/internal/opensky"
)

// -------------------------------------------------------------------------
// TEST HELPERS
// -------------------------------------------------------------------------

// stubSource is a minimal GlobalFlightSource for testing.
type stubSource struct {
	resp *opensky.StatesResponse
	err  error
}

// GetStates returns the stubbed response.
func (s *stubSource) GetStates(_ context.Context, _ geo.BBox) (*opensky.StatesResponse, error) {
	return s.resp, s.err
}

// stubAlertStore records calls to InsertSquawkAlert.
type stubAlertStore struct {
	alerts []insertCall
	err    error
}

// insertCall records the arguments to a single InsertSquawkAlert call.
type insertCall struct {
	icao24, callsign, squawk string
}

// InsertSquawkAlert records the call and returns the stubbed error.
func (s *stubAlertStore) InsertSquawkAlert(_ context.Context, icao24, callsign, squawk string, _, _ float64) error {
	s.alerts = append(s.alerts, insertCall{icao24, callsign, squawk})
	return s.err
}

// stubEnricher records enrichment calls.
type stubEnricher struct {
	enriched []string
	routes   []string
}

// Enrich records the ICAO24.
func (s *stubEnricher) Enrich(_ context.Context, icao24 string) bool {
	s.enriched = append(s.enriched, icao24)
	return true
}

// EnrichRoute records the callsign.
func (s *stubEnricher) EnrichRoute(_ context.Context, callsign string) {
	s.routes = append(s.routes, callsign)
}

// -------------------------------------------------------------------------
// TESTS
// -------------------------------------------------------------------------

// TestScan_DetectsEmergencySquawks verifies that emergency codes are detected and stored.
func TestScan_DetectsEmergencySquawks(t *testing.T) {
	source := &stubSource{resp: &opensky.StatesResponse{
		Time: 1234,
		States: []opensky.StateVector{
			{ICAO24: "a1", Callsign: "UAL123", Squawk: "7700", Latitude: 34.0, Longitude: -118.0},
			{ICAO24: "a2", Callsign: "DAL456", Squawk: "1200", Latitude: 35.0, Longitude: -119.0},
			{ICAO24: "a3", Callsign: "AAL789", Squawk: "7500", Latitude: 36.0, Longitude: -120.0},
		},
	}}
	store := &stubAlertStore{}
	enr := &stubEnricher{}

	m := New(source, store, enr, time.Minute)
	m.scan(context.Background())

	if len(store.alerts) != 2 {
		t.Fatalf("len(alerts) = %d, want 2", len(store.alerts))
	}
	if store.alerts[0].icao24 != "a1" || store.alerts[0].squawk != "7700" {
		t.Errorf("alert[0] = %+v, want icao24=a1 squawk=7700", store.alerts[0])
	}
	if store.alerts[1].icao24 != "a3" || store.alerts[1].squawk != "7500" {
		t.Errorf("alert[1] = %+v, want icao24=a3 squawk=7500", store.alerts[1])
	}
	if len(enr.enriched) != 2 {
		t.Errorf("len(enriched) = %d, want 2", len(enr.enriched))
	}
	if len(enr.routes) != 2 {
		t.Errorf("len(routes) = %d, want 2", len(enr.routes))
	}
}

// TestScan_NoEmergencies verifies that a response with no emergency squawks produces no alerts.
func TestScan_NoEmergencies(t *testing.T) {
	source := &stubSource{resp: &opensky.StatesResponse{
		Time: 1234,
		States: []opensky.StateVector{
			{ICAO24: "a1", Squawk: "1200"},
			{ICAO24: "a2", Squawk: "4512"},
			{ICAO24: "a3", Squawk: ""},
		},
	}}
	store := &stubAlertStore{}
	enr := &stubEnricher{}

	m := New(source, store, enr, time.Minute)
	m.scan(context.Background())

	if len(store.alerts) != 0 {
		t.Errorf("len(alerts) = %d, want 0", len(store.alerts))
	}
}

// TestScan_EmptyResponse verifies that an empty states response completes without errors.
func TestScan_EmptyResponse(t *testing.T) {
	source := &stubSource{resp: &opensky.StatesResponse{Time: 1234}}
	store := &stubAlertStore{}
	enr := &stubEnricher{}

	m := New(source, store, enr, time.Minute)
	m.scan(context.Background())

	if len(store.alerts) != 0 {
		t.Errorf("len(alerts) = %d, want 0", len(store.alerts))
	}
}

// TestScan_SourceError verifies that a source error is handled gracefully.
func TestScan_SourceError(t *testing.T) {
	source := &stubSource{err: errors.New("api down")}
	store := &stubAlertStore{}
	enr := &stubEnricher{}

	m := New(source, store, enr, time.Minute)
	m.scan(context.Background())

	if len(store.alerts) != 0 {
		t.Errorf("len(alerts) = %d, want 0", len(store.alerts))
	}
}

// TestScan_StoreError verifies that a store error does not stop processing other alerts.
func TestScan_StoreError(t *testing.T) {
	source := &stubSource{resp: &opensky.StatesResponse{
		Time: 1234,
		States: []opensky.StateVector{
			{ICAO24: "a1", Callsign: "UAL123", Squawk: "7700"},
			{ICAO24: "a2", Callsign: "DAL456", Squawk: "7600"},
		},
	}}
	store := &stubAlertStore{err: errors.New("pg down")}
	enr := &stubEnricher{}

	m := New(source, store, enr, time.Minute)
	m.scan(context.Background())

	// Both should be attempted even though store errors
	if len(store.alerts) != 2 {
		t.Errorf("len(alerts) = %d, want 2", len(store.alerts))
	}
	if len(enr.enriched) != 2 {
		t.Errorf("len(enriched) = %d, want 2", len(enr.enriched))
	}
}

// TestScan_EmptyCallsign verifies that empty callsigns skip route enrichment.
func TestScan_EmptyCallsign(t *testing.T) {
	source := &stubSource{resp: &opensky.StatesResponse{
		Time: 1234,
		States: []opensky.StateVector{
			{ICAO24: "a1", Callsign: "", Squawk: "7700"},
		},
	}}
	store := &stubAlertStore{}
	enr := &stubEnricher{}

	m := New(source, store, enr, time.Minute)
	m.scan(context.Background())

	if len(enr.enriched) != 1 {
		t.Errorf("len(enriched) = %d, want 1", len(enr.enriched))
	}
	if len(enr.routes) != 0 {
		t.Errorf("len(routes) = %d, want 0 for empty callsign", len(enr.routes))
	}
}

// TestScan_DeduplicatesWithinCooldown verifies that the same aircraft+squawk is only recorded once within the cooldown window.
func TestScan_DeduplicatesWithinCooldown(t *testing.T) {
	source := &stubSource{resp: &opensky.StatesResponse{
		Time: 1234,
		States: []opensky.StateVector{
			{ICAO24: "a1", Callsign: "UAL123", Squawk: "7700", Latitude: 34.0, Longitude: -118.0},
		},
	}}
	store := &stubAlertStore{}
	enr := &stubEnricher{}

	m := New(source, store, enr, time.Minute)
	m.scan(context.Background())
	m.scan(context.Background())
	m.scan(context.Background())

	if len(store.alerts) != 1 {
		t.Errorf("len(alerts) = %d, want 1 (duplicates should be suppressed)", len(store.alerts))
	}
	if len(enr.enriched) != 1 {
		t.Errorf("len(enriched) = %d, want 1", len(enr.enriched))
	}
}
