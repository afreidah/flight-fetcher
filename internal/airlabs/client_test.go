// -------------------------------------------------------------------------------
// AirLabs - Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests the AirLabs API client: successful route lookups, empty callsigns,
// not-found responses, and error handling.
// -------------------------------------------------------------------------------

package airlabs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/afreidah/flight-fetcher/internal/apiclient"
)

// testClient creates a Client pointed at the given test server.
func testClient(srv *httptest.Server, apiKey string) *Client {
	return &Client{
		Client: apiclient.New(apiclient.Options{BaseURL: srv.URL}),
		apiKey: apiKey,
	}
}

// TestLookupRoute_Success verifies that a valid response is decoded correctly.
func TestLookupRoute_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("flight_icao") != "AAL2079" {
			t.Errorf("unexpected flight_icao: %s", r.URL.Query().Get("flight_icao"))
		}
		if r.URL.Query().Get("api_key") != "test-key" {
			t.Errorf("unexpected api_key: %s", r.URL.Query().Get("api_key"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"response": {
				"flight_icao": "AAL2079",
				"flight_iata": "AA2079",
				"dep_iata": "LAX",
				"dep_icao": "KLAX",
				"dep_name": "Los Angeles International Airport",
				"arr_iata": "DFW",
				"arr_icao": "KDFW",
				"arr_name": "Dallas/Fort Worth International Airport"
			}
		}`))
	}))
	defer srv.Close()

	c := testClient(srv, "test-key")
	route, err := c.LookupRoute(context.Background(), "AAL2079")
	if err != nil {
		t.Fatalf("LookupRoute() error = %v", err)
	}
	if route == nil {
		t.Fatal("LookupRoute() returned nil")
	}
	if route.DepIATA != "LAX" {
		t.Errorf("DepIATA = %q, want %q", route.DepIATA, "LAX")
	}
	if route.DepICAO != "KLAX" {
		t.Errorf("DepICAO = %q, want %q", route.DepICAO, "KLAX")
	}
	if route.ArrIATA != "DFW" {
		t.Errorf("ArrIATA = %q, want %q", route.ArrIATA, "DFW")
	}
	if route.ArrICAO != "KDFW" {
		t.Errorf("ArrICAO = %q, want %q", route.ArrICAO, "KDFW")
	}
	if route.DepName != "Los Angeles International Airport" {
		t.Errorf("DepName = %q", route.DepName)
	}
	if route.ArrName != "Dallas/Fort Worth International Airport" {
		t.Errorf("ArrName = %q", route.ArrName)
	}
}

// TestLookupRoute_EmptyCallsign verifies that an empty callsign returns nil.
func TestLookupRoute_EmptyCallsign(t *testing.T) {
	c := NewClient("test-key")
	route, err := c.LookupRoute(context.Background(), "")
	if err != nil {
		t.Fatalf("LookupRoute() error = %v", err)
	}
	if route != nil {
		t.Errorf("LookupRoute() = %v, want nil for empty callsign", route)
	}
}

// TestLookupRoute_NotFound verifies that an empty API response returns nil.
func TestLookupRoute_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response": {}}`))
	}))
	defer srv.Close()

	c := testClient(srv, "test-key")
	route, err := c.LookupRoute(context.Background(), "UNKNOWN")
	if err != nil {
		t.Fatalf("LookupRoute() error = %v", err)
	}
	if route != nil {
		t.Errorf("LookupRoute() = %v, want nil for unknown flight", route)
	}
}

// TestLookupRoute_ServerError verifies that a non-200 response returns an error.
func TestLookupRoute_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := testClient(srv, "test-key")
	_, err := c.LookupRoute(context.Background(), "AAL2079")
	if err == nil {
		t.Error("LookupRoute() expected error for 500 response, got nil")
	}
}

// TestLookupRoute_InvalidJSON verifies that a malformed response returns an error.
func TestLookupRoute_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	c := testClient(srv, "test-key")
	_, err := c.LookupRoute(context.Background(), "AAL2079")
	if err == nil {
		t.Error("LookupRoute() expected error for invalid JSON, got nil")
	}
}
