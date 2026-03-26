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

	"time"

	"github.com/afreidah/flight-fetcher/internal/aircraft"
	"github.com/afreidah/flight-fetcher/internal/route"
	"github.com/afreidah/flight-fetcher/internal/apiclient/opensky"
	"github.com/afreidah/flight-fetcher/internal/squawk"
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
	info *aircraft.Info
	err  error
}

// GetAircraftMeta returns the stubbed aircraft metadata.
func (s *stubMetaReader) GetAircraftMeta(_ context.Context, _ string) (*aircraft.Info, error) {
	return s.info, s.err
}

// stubRouteReader is a minimal RouteReader for testing.
type stubRouteReader struct {
	info *route.Info
	err   error
}

// GetFlightRoute returns the stubbed flight route.
func (s *stubRouteReader) GetFlightRoute(_ context.Context, _ string) (*route.Info, error) {
	return s.info, s.err
}

// stubAlertReader is a minimal SquawkAlertReader for testing.
type stubAlertReader struct {
	alerts []squawk.Alert
	err    error
}

// GetRecentSquawkAlerts returns the stubbed alerts.
func (s *stubAlertReader) GetRecentSquawkAlerts(_ context.Context, _ time.Duration) ([]squawk.Alert, error) {
	return s.alerts, s.err
}

// stubImageFetcher is a minimal ImageFetcher for testing.
type stubImageFetcher struct {
	url string
}

// FetchImageURL returns the stubbed URL.
func (s *stubImageFetcher) FetchImageURL(_ context.Context, _ string) string {
	return s.url
}

// stubPinger is a minimal HealthPinger for testing.
type stubPinger struct {
	err error
}

// Ping returns the stubbed error.
func (s *stubPinger) Ping(_ context.Context) error {
	return s.err
}

// -------------------------------------------------------------------------
// TESTS
// -------------------------------------------------------------------------

// TestHandleIndex verifies that the index page returns HTML.
func TestHandleIndex(t *testing.T) {
	srv := New(&Options{Flights: &stubFlightLister{}, Aircraft: &stubMetaReader{}, Version: "test", RefreshSec: 5})
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
	srv := New(&Options{Flights: &stubFlightLister{flights: flights}, Aircraft: &stubMetaReader{}, Version: "test", RefreshSec: 5})
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

// TestHandleListFlights_WithClassification verifies that classification and operator code are included.
func TestHandleListFlights_WithClassification(t *testing.T) {
	flights := []opensky.StateVector{
		{ICAO24: "abc123", Callsign: "UAL123"},
	}
	meta := &aircraft.Info{ICAO24: "abc123", Registration: "N12345", OperatorFlagCode: "UAL", RegisteredOwners: "United Airlines"}
	srv := New(&Options{Flights: &stubFlightLister{flights: flights}, Aircraft: &stubMetaReader{info: meta}, Version: "test", RefreshSec: 5})
	req := httptest.NewRequest(http.MethodGet, "/api/flights", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	var got []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0]["operator_code"] != "UAL" {
		t.Errorf("operator_code = %v, want %q", got[0]["operator_code"], "UAL")
	}
}

// TestHandleListFlights_Error verifies that a store error degrades to an empty array instead of 500.
func TestHandleListFlights_Error(t *testing.T) {
	srv := New(&Options{Flights: &stubFlightLister{err: errors.New("redis down")}, Aircraft: &stubMetaReader{}, Version: "test", RefreshSec: 5})
	req := httptest.NewRequest(http.MethodGet, "/api/flights", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (should degrade gracefully)", w.Code, http.StatusOK)
	}
	var got []opensky.StateVector
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(flights) = %d, want 0 on error", len(got))
	}
}

// TestHandleListFlights_Empty verifies that an empty flight list returns an empty JSON array.
func TestHandleListFlights_Empty(t *testing.T) {
	srv := New(&Options{Flights: &stubFlightLister{flights: []opensky.StateVector{}}, Aircraft: &stubMetaReader{}, Version: "test", RefreshSec: 5})
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
	meta := &aircraft.Info{ICAO24: "abc123", Registration: "N12345", ManufacturerName: "Boeing", Type: "737-800"}
	srv := New(&Options{Flights: &stubFlightLister{flight: sv}, Aircraft: &stubMetaReader{info: meta}, Version: "test", RefreshSec: 5})
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
	ri := &route.Info{FlightICAO: "AAL2079", DepIATA: "LAX", ArrIATA: "DFW"}
	srv := New(&Options{Flights: &stubFlightLister{flight: sv}, Aircraft: &stubMetaReader{}, Routes: &stubRouteReader{info: ri}, Version: "test", RefreshSec: 5})
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
	srv := New(&Options{Flights: &stubFlightLister{flight: sv}, Aircraft: &stubMetaReader{info: nil}, Version: "test", RefreshSec: 5})
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
	srv := New(&Options{Flights: &stubFlightLister{flight: nil}, Aircraft: &stubMetaReader{}, Version: "test", RefreshSec: 5})
	req := httptest.NewRequest(http.MethodGet, "/api/flights/unknown", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// TestHandleGetFlight_StoreError verifies that a store error returns 500.
func TestHandleGetFlight_StoreError(t *testing.T) {
	srv := New(&Options{Flights: &stubFlightLister{err: errors.New("redis down")}, Aircraft: &stubMetaReader{}, Version: "test", RefreshSec: 5})
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
	srv := New(&Options{Flights: &stubFlightLister{flight: sv}, Aircraft: &stubMetaReader{err: errors.New("pg down")}, Version: "test", RefreshSec: 5})
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
	srv := New(&Options{Flights: &stubFlightLister{flight: sv}, Aircraft: &stubMetaReader{}, Routes: &stubRouteReader{err: errors.New("pg down")}, Version: "test", RefreshSec: 5})
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

// TestHandleSquawkAlerts_Success verifies that squawk alerts are returned as JSON.
func TestHandleSquawkAlerts_Success(t *testing.T) {
	alerts := []squawk.Alert{
		{ID: 1, ICAO24: "a1", Callsign: "UAL123", Squawk: "7700", Lat: 34.0, Lon: -118.0,
			SeenAt: time.Now().UTC()},
	}
	srv := New(&Options{Flights: &stubFlightLister{}, Aircraft: &stubMetaReader{}, Alerts: &stubAlertReader{alerts: alerts}, Version: "test", RefreshSec: 5})
	req := httptest.NewRequest(http.MethodGet, "/api/squawk-alerts", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var got []squawk.Alert
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len(alerts) = %d, want 1", len(got))
	}
}

// TestHandleSquawkAlerts_Disabled verifies that nil alerts reader returns empty array.
func TestHandleSquawkAlerts_Disabled(t *testing.T) {
	srv := New(&Options{Flights: &stubFlightLister{}, Aircraft: &stubMetaReader{}, Version: "test", RefreshSec: 5})
	req := httptest.NewRequest(http.MethodGet, "/api/squawk-alerts", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// TestHandleSquawkAlerts_Error verifies that a store error returns 500.
func TestHandleSquawkAlerts_Error(t *testing.T) {
	srv := New(&Options{Flights: &stubFlightLister{}, Aircraft: &stubMetaReader{}, Alerts: &stubAlertReader{err: errors.New("pg down")}, Version: "test", RefreshSec: 5})
	req := httptest.NewRequest(http.MethodGet, "/api/squawk-alerts", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

// TestHandleGetAircraft_Success verifies that aircraft metadata is returned.
func TestHandleGetAircraft_Success(t *testing.T) {
	meta := &aircraft.Info{ICAO24: "abc123", Registration: "N12345", ManufacturerName: "Boeing"}
	srv := New(&Options{Flights: &stubFlightLister{}, Aircraft: &stubMetaReader{info: meta}, Version: "test", RefreshSec: 5})
	req := httptest.NewRequest(http.MethodGet, "/api/aircraft/abc123", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var got aircraft.Info
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got.Registration != "N12345" {
		t.Errorf("Registration = %q, want %q", got.Registration, "N12345")
	}
}

// TestHandleGetAircraft_NotFound verifies that a missing aircraft returns 404.
func TestHandleGetAircraft_NotFound(t *testing.T) {
	srv := New(&Options{Flights: &stubFlightLister{}, Aircraft: &stubMetaReader{}, Version: "test", RefreshSec: 5})
	req := httptest.NewRequest(http.MethodGet, "/api/aircraft/unknown", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// TestHandleGetRoute_Success verifies that route data is returned.
func TestHandleGetRoute_Success(t *testing.T) {
	ri := &route.Info{FlightICAO: "AAL2079", DepIATA: "LAX", ArrIATA: "DFW"}
	srv := New(&Options{Flights: &stubFlightLister{}, Aircraft: &stubMetaReader{}, Routes: &stubRouteReader{info: ri}, Version: "test", RefreshSec: 5})
	req := httptest.NewRequest(http.MethodGet, "/api/routes/AAL2079", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var got route.Info
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got.DepIATA != "LAX" {
		t.Errorf("DepIATA = %q, want %q", got.DepIATA, "LAX")
	}
}

// TestHandleGetRoute_NotFound verifies that a missing route returns 404.
func TestHandleGetRoute_NotFound(t *testing.T) {
	srv := New(&Options{Flights: &stubFlightLister{}, Aircraft: &stubMetaReader{}, Routes: &stubRouteReader{}, Version: "test", RefreshSec: 5})
	req := httptest.NewRequest(http.MethodGet, "/api/routes/UNKNOWN", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// TestHandleGetRoute_Disabled verifies that nil routes reader returns 404.
func TestHandleGetRoute_Disabled(t *testing.T) {
	srv := New(&Options{Flights: &stubFlightLister{}, Aircraft: &stubMetaReader{}, Version: "test", RefreshSec: 5})
	req := httptest.NewRequest(http.MethodGet, "/api/routes/AAL2079", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// TestHandleGetAircraft_Sentinel verifies that a sentinel record (empty fields) returns 404.
func TestHandleGetAircraft_Sentinel(t *testing.T) {
	sentinel := &aircraft.Info{ICAO24: "abc123"}
	srv := New(&Options{Flights: &stubFlightLister{}, Aircraft: &stubMetaReader{info: sentinel}, Version: "test", RefreshSec: 5})
	req := httptest.NewRequest(http.MethodGet, "/api/aircraft/abc123", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d (sentinel should be treated as not found)", w.Code, http.StatusNotFound)
	}
}

// TestHandleGetAircraft_Error verifies that a store error returns 500.
func TestHandleGetAircraft_Error(t *testing.T) {
	srv := New(&Options{Flights: &stubFlightLister{}, Aircraft: &stubMetaReader{err: errors.New("pg down")}, Version: "test", RefreshSec: 5})
	req := httptest.NewRequest(http.MethodGet, "/api/aircraft/abc123", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

// TestHandleGetFlight_ImageBackfill verifies that a missing image URL is fetched on demand.
func TestHandleGetFlight_ImageBackfill(t *testing.T) {
	sv := &opensky.StateVector{ICAO24: "abc123", Callsign: "UAL123"}
	meta := &aircraft.Info{ICAO24: "abc123", Registration: "N12345", Type: "737-800"}
	srv := New(&Options{
		Flights:  &stubFlightLister{flight: sv},
		Aircraft: &stubMetaReader{info: meta},
		Images:   &stubImageFetcher{url: "https://hexdb.io/static/aircraft-images/N12345.jpg"},
		Version:  "test", RefreshSec: 5,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/flights/abc123", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	var detail flightDetail
	if err := json.NewDecoder(w.Body).Decode(&detail); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if detail.Aircraft.ImageURL != "https://hexdb.io/static/aircraft-images/N12345.jpg" {
		t.Errorf("ImageURL = %q, want backfilled URL", detail.Aircraft.ImageURL)
	}
}

// TestHandleGetAircraft_ImageBackfill verifies that a missing image URL is fetched on the aircraft endpoint.
func TestHandleGetAircraft_ImageBackfill(t *testing.T) {
	meta := &aircraft.Info{ICAO24: "abc123", Registration: "N12345", Type: "737-800"}
	srv := New(&Options{
		Flights:  &stubFlightLister{},
		Aircraft: &stubMetaReader{info: meta},
		Images:   &stubImageFetcher{url: "https://hexdb.io/static/aircraft-images/N12345.jpg"},
		Version:  "test", RefreshSec: 5,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/aircraft/abc123", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	var info aircraft.Info
	if err := json.NewDecoder(w.Body).Decode(&info); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if info.ImageURL != "https://hexdb.io/static/aircraft-images/N12345.jpg" {
		t.Errorf("ImageURL = %q, want backfilled URL", info.ImageURL)
	}
}

// TestHandleHealthz_Healthy verifies that all-healthy pingers return 200 with status "healthy".
func TestHandleHealthz_Healthy(t *testing.T) {
	srv := New(&Options{
		Flights:  &stubFlightLister{},
		Aircraft: &stubMetaReader{},
		Pingers: []HealthPinger{
			{Name: "redis", Pinger: &stubPinger{}},
			{Name: "postgres", Pinger: &stubPinger{}},
		},
		Version: "test",
	})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var got healthResponse
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got.Status != "healthy" {
		t.Errorf("status = %q, want %q", got.Status, "healthy")
	}
	if got.Components["redis"] != "ok" {
		t.Errorf("redis = %q, want %q", got.Components["redis"], "ok")
	}
}

// TestHandleHealthz_Degraded verifies that one failing pinger returns 200 with status "degraded".
func TestHandleHealthz_Degraded(t *testing.T) {
	srv := New(&Options{
		Flights:  &stubFlightLister{},
		Aircraft: &stubMetaReader{},
		Pingers: []HealthPinger{
			{Name: "redis", Pinger: &stubPinger{}},
			{Name: "postgres", Pinger: &stubPinger{err: errors.New("pg down")}},
		},
		Version: "test",
	})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (degraded should be 200)", w.Code, http.StatusOK)
	}
	var got healthResponse
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got.Status != "degraded" {
		t.Errorf("status = %q, want %q", got.Status, "degraded")
	}
	if got.Components["redis"] != "ok" {
		t.Errorf("redis = %q, want %q", got.Components["redis"], "ok")
	}
	if got.Components["postgres"] != "pg down" {
		t.Errorf("postgres = %q, want %q", got.Components["postgres"], "pg down")
	}
}

// TestHandleHealthz_Unhealthy verifies that all-failing pingers return 503.
func TestHandleHealthz_Unhealthy(t *testing.T) {
	srv := New(&Options{
		Flights:  &stubFlightLister{},
		Aircraft: &stubMetaReader{},
		Pingers: []HealthPinger{
			{Name: "redis", Pinger: &stubPinger{err: errors.New("redis down")}},
			{Name: "postgres", Pinger: &stubPinger{err: errors.New("pg down")}},
		},
		Version: "test",
	})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
	var got healthResponse
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got.Status != "unhealthy" {
		t.Errorf("status = %q, want %q", got.Status, "unhealthy")
	}
}

// TestHandleHealthz_NoPingers verifies that no pingers returns 200 with status "healthy".
func TestHandleHealthz_NoPingers(t *testing.T) {
	srv := New(&Options{
		Flights:  &stubFlightLister{},
		Aircraft: &stubMetaReader{},
		Version:  "test",
	})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

