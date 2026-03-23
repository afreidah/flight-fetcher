// -------------------------------------------------------------------------------
// Server - Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests the dashboard HTTP handlers: flight listing, flight detail with enriched
// metadata and route information, missing flights, and error handling.
// -------------------------------------------------------------------------------

package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/afreidah/flight-fetcher/internal/airlabs"
	"github.com/afreidah/flight-fetcher/internal/hexdb"
	"github.com/afreidah/flight-fetcher/internal/opensky"
)

// -------------------------------------------------------------------------
// TEST HELPERS
// -------------------------------------------------------------------------

// stubFlightLister is a minimal FlightLister for testing.
type stubFlightLister struct {
	flights []opensky.StateVector
	flight  *opensky.StateVector
	err     error
}

// GetAllFlights returns the stubbed flight list.
func (s *stubFlightLister) GetAllFlights(_ context.Context) ([]opensky.StateVector, error) {
	return s.flights, s.err
}

// GetFlight returns the stubbed single flight.
func (s *stubFlightLister) GetFlight(_ context.Context, _ string) (*opensky.StateVector, error) {
	return s.flight, s.err
}

// stubMetaReader is a minimal AircraftMetaReader for testing.
type stubMetaReader struct {
	info *hexdb.AircraftInfo
	err  error
}

// GetAircraftMeta returns the stubbed aircraft metadata.
func (s *stubMetaReader) GetAircraftMeta(_ context.Context, _ string) (*hexdb.AircraftInfo, error) {
	return s.info, s.err
}

// stubRouteReader is a minimal RouteReader for testing.
type stubRouteReader struct {
	route *airlabs.FlightRoute
	err   error
}

// GetFlightRoute returns the stubbed flight route.
func (s *stubRouteReader) GetFlightRoute(_ context.Context, _ string) (*airlabs.FlightRoute, error) {
	return s.route, s.err
}

// -------------------------------------------------------------------------
// TESTS
// -------------------------------------------------------------------------

// TestHandleIndex verifies that the index page returns HTML.
func TestHandleIndex(t *testing.T) {
	srv := New(&stubFlightLister{}, &stubMetaReader{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	if w.Body.Len() == 0 {
		t.Error("expected non-empty body")
	}
}

// TestHandleListFlights_Success verifies that the flight list endpoint returns JSON.
func TestHandleListFlights_Success(t *testing.T) {
	flights := []opensky.StateVector{
		{ICAO24: "abc123", Callsign: "UAL123", Latitude: 34.09, Longitude: -118.33},
		{ICAO24: "def456", Callsign: "DAL456", Latitude: 34.10, Longitude: -118.34},
	}
	srv := New(&stubFlightLister{flights: flights}, &stubMetaReader{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/flights", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var got []opensky.StateVector
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(flights) = %d, want 2", len(got))
	}
}

// TestHandleListFlights_Error verifies that a store error returns 500.
func TestHandleListFlights_Error(t *testing.T) {
	srv := New(&stubFlightLister{err: errors.New("redis down")}, &stubMetaReader{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/flights", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

// TestHandleListFlights_Empty verifies that an empty flight list returns an empty JSON array.
func TestHandleListFlights_Empty(t *testing.T) {
	srv := New(&stubFlightLister{flights: []opensky.StateVector{}}, &stubMetaReader{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/flights", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var got []opensky.StateVector
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(flights) = %d, want 0", len(got))
	}
}

// TestHandleGetFlight_WithMeta verifies that flight detail includes enriched metadata.
func TestHandleGetFlight_WithMeta(t *testing.T) {
	sv := &opensky.StateVector{ICAO24: "abc123", Callsign: "UAL123", Latitude: 34.09, Longitude: -118.33}
	meta := &hexdb.AircraftInfo{ICAO24: "abc123", Registration: "N12345", ManufacturerName: "Boeing", Type: "737-800"}
	srv := New(
		&stubFlightLister{flight: sv},
		&stubMetaReader{info: meta},
		nil,
	)
	req := httptest.NewRequest(http.MethodGet, "/api/flights/abc123", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var got flightDetail
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got.State.ICAO24 != "abc123" {
		t.Errorf("State.ICAO24 = %q, want %q", got.State.ICAO24, "abc123")
	}
	if got.Aircraft == nil {
		t.Fatal("Aircraft should not be nil")
	}
	if got.Aircraft.Registration != "N12345" {
		t.Errorf("Aircraft.Registration = %q, want %q", got.Aircraft.Registration, "N12345")
	}
}

// TestHandleGetFlight_WithRoute verifies that flight detail includes route information.
func TestHandleGetFlight_WithRoute(t *testing.T) {
	sv := &opensky.StateVector{ICAO24: "abc123", Callsign: "AAL2079"}
	route := &airlabs.FlightRoute{FlightICAO: "AAL2079", DepIATA: "LAX", ArrIATA: "DFW"}
	srv := New(
		&stubFlightLister{flight: sv},
		&stubMetaReader{},
		&stubRouteReader{route: route},
	)
	req := httptest.NewRequest(http.MethodGet, "/api/flights/abc123", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var got flightDetail
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got.Route == nil {
		t.Fatal("Route should not be nil")
	}
	if got.Route.DepIATA != "LAX" {
		t.Errorf("Route.DepIATA = %q, want %q", got.Route.DepIATA, "LAX")
	}
	if got.Route.ArrIATA != "DFW" {
		t.Errorf("Route.ArrIATA = %q, want %q", got.Route.ArrIATA, "DFW")
	}
}

// TestHandleGetFlight_NoMeta verifies that flight detail works when no enriched metadata exists.
func TestHandleGetFlight_NoMeta(t *testing.T) {
	sv := &opensky.StateVector{ICAO24: "abc123", Callsign: "UAL123"}
	srv := New(
		&stubFlightLister{flight: sv},
		&stubMetaReader{info: nil},
		nil,
	)
	req := httptest.NewRequest(http.MethodGet, "/api/flights/abc123", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var got flightDetail
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got.Aircraft != nil {
		t.Errorf("Aircraft should be nil when not enriched, got %v", got.Aircraft)
	}
}

// TestHandleGetFlight_NotFound verifies that a missing flight returns 404.
func TestHandleGetFlight_NotFound(t *testing.T) {
	srv := New(&stubFlightLister{flight: nil}, &stubMetaReader{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/flights/unknown", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// TestHandleGetFlight_StoreError verifies that a store error returns 500.
func TestHandleGetFlight_StoreError(t *testing.T) {
	srv := New(&stubFlightLister{err: errors.New("redis down")}, &stubMetaReader{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/flights/abc123", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

// TestHandleGetFlight_MetaError verifies that a metadata lookup failure still returns the flight state.
func TestHandleGetFlight_MetaError(t *testing.T) {
	sv := &opensky.StateVector{ICAO24: "abc123"}
	srv := New(
		&stubFlightLister{flight: sv},
		&stubMetaReader{err: errors.New("pg down")},
		nil,
	)
	req := httptest.NewRequest(http.MethodGet, "/api/flights/abc123", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var got flightDetail
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got.State.ICAO24 != "abc123" {
		t.Errorf("State.ICAO24 = %q, want %q", got.State.ICAO24, "abc123")
	}
	if got.Aircraft != nil {
		t.Errorf("Aircraft should be nil on meta error, got %v", got.Aircraft)
	}
}

// TestHandleGetFlight_RouteError verifies that a route lookup failure still returns the flight state.
func TestHandleGetFlight_RouteError(t *testing.T) {
	sv := &opensky.StateVector{ICAO24: "abc123", Callsign: "AAL2079"}
	srv := New(
		&stubFlightLister{flight: sv},
		&stubMetaReader{},
		&stubRouteReader{err: errors.New("pg down")},
	)
	req := httptest.NewRequest(http.MethodGet, "/api/flights/abc123", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var got flightDetail
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got.Route != nil {
		t.Errorf("Route should be nil on route error, got %v", got.Route)
	}
}
