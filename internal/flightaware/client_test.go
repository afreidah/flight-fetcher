// -------------------------------------------------------------------------------
// FlightAware - Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests the FlightAware AeroAPI client: successful route lookups, empty
// callsigns, not-found responses, empty flight lists, and error handling.
// -------------------------------------------------------------------------------

package flightaware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestLookupRoute_Success verifies that a valid response is decoded correctly.
func TestLookupRoute_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/flights/SWA964" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-apikey") != "test-key" {
			t.Errorf("unexpected apikey: %s", r.Header.Get("x-apikey"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"flights": [{
				"ident": "SWA964",
				"origin": {
					"code_icao": "KLAX",
					"code_iata": "LAX",
					"name": "Los Angeles International Airport"
				},
				"destination": {
					"code_icao": "KLAS",
					"code_iata": "LAS",
					"name": "Harry Reid International Airport"
				}
			}]
		}`))
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), apiKey: "test-key", baseURL: srv.URL}
	route, err := c.LookupRoute(context.Background(), "SWA964")
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
	if route.ArrIATA != "LAS" {
		t.Errorf("ArrIATA = %q, want %q", route.ArrIATA, "LAS")
	}
	if route.ArrName != "Harry Reid International Airport" {
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

// TestLookupRoute_NotFound verifies that a 404 response returns nil.
func TestLookupRoute_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), apiKey: "test-key", baseURL: srv.URL}
	route, err := c.LookupRoute(context.Background(), "UNKNOWN")
	if err != nil {
		t.Fatalf("LookupRoute() error = %v", err)
	}
	if route != nil {
		t.Errorf("LookupRoute() = %v, want nil for 404", route)
	}
}

// TestLookupRoute_EmptyFlights verifies that an empty flights array returns nil.
func TestLookupRoute_EmptyFlights(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"flights": []}`))
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), apiKey: "test-key", baseURL: srv.URL}
	route, err := c.LookupRoute(context.Background(), "SWA964")
	if err != nil {
		t.Fatalf("LookupRoute() error = %v", err)
	}
	if route != nil {
		t.Errorf("LookupRoute() = %v, want nil for empty flights", route)
	}
}

// TestLookupRoute_ServerError verifies that a 500 response returns an error.
func TestLookupRoute_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), apiKey: "test-key", baseURL: srv.URL}
	_, err := c.LookupRoute(context.Background(), "SWA964")
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

	c := &Client{httpClient: srv.Client(), apiKey: "test-key", baseURL: srv.URL}
	_, err := c.LookupRoute(context.Background(), "SWA964")
	if err == nil {
		t.Error("LookupRoute() expected error for invalid JSON, got nil")
	}
}
