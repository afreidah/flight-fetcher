// -------------------------------------------------------------------------------
// OpenSky - Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests the OpenSky API client: state vector parsing from raw JSON arrays,
// full HTTP response decoding, and error handling.
// -------------------------------------------------------------------------------

package opensky

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/afreidah/flight-fetcher/internal/geo"
)

// TestParseStateVector_Valid verifies that a complete state vector array is parsed correctly.
func TestParseStateVector_Valid(t *testing.T) {
	raw := []interface{}{
		"abc123",        // 0: icao24
		"UAL123  ",      // 1: callsign
		"United States", // 2: origin_country
		float64(1234),   // 3: time_position
		float64(1234),   // 4: last_contact
		-118.4081,       // 5: longitude
		33.9425,         // 6: latitude
		3048.0,          // 7: baro_altitude
		false,           // 8: on_ground
		125.5,           // 9: velocity
		270.0,           // 10: heading
		-5.2,            // 11: vertical_rate
		nil,             // 12: sensors
		3100.0,          // 13: geo_altitude
		"1234",          // 14: squawk
		false,           // 15: spi
		float64(0),      // 16: position_source
	}

	sv, err := parseStateVector(raw)
	if err != nil {
		t.Fatalf("parseStateVector() error = %v", err)
	}
	if sv.ICAO24 != "abc123" {
		t.Errorf("ICAO24 = %q, want %q", sv.ICAO24, "abc123")
	}
	if sv.Callsign != "UAL123  " {
		t.Errorf("Callsign = %q, want %q", sv.Callsign, "UAL123  ")
	}
	if sv.OriginCountry != "United States" {
		t.Errorf("OriginCountry = %q, want %q", sv.OriginCountry, "United States")
	}
	if sv.Longitude != -118.4081 {
		t.Errorf("Longitude = %f, want %f", sv.Longitude, -118.4081)
	}
	if sv.Latitude != 33.9425 {
		t.Errorf("Latitude = %f, want %f", sv.Latitude, 33.9425)
	}
	if sv.BaroAltitude != 3048.0 {
		t.Errorf("BaroAltitude = %f, want %f", sv.BaroAltitude, 3048.0)
	}
	if sv.Velocity != 125.5 {
		t.Errorf("Velocity = %f, want %f", sv.Velocity, 125.5)
	}
	if sv.Heading != 270.0 {
		t.Errorf("Heading = %f, want %f", sv.Heading, 270.0)
	}
	if sv.VerticalRate != -5.2 {
		t.Errorf("VerticalRate = %f, want %f", sv.VerticalRate, -5.2)
	}
	if sv.OnGround != false {
		t.Errorf("OnGround = %v, want false", sv.OnGround)
	}
}

// TestParseStateVector_TooShort verifies that an undersized array returns an error.
func TestParseStateVector_TooShort(t *testing.T) {
	raw := []interface{}{"abc123", "UAL123"}
	_, err := parseStateVector(raw)
	if err == nil {
		t.Error("parseStateVector() expected error for short array, got nil")
	}
}

// TestParseStateVector_NullFields verifies that nil fields default to zero values.
func TestParseStateVector_NullFields(t *testing.T) {
	raw := make([]interface{}, 17)
	raw[0] = "abc123"
	raw[2] = "United States"

	sv, err := parseStateVector(raw)
	if err != nil {
		t.Fatalf("parseStateVector() error = %v", err)
	}
	if sv.ICAO24 != "abc123" {
		t.Errorf("ICAO24 = %q, want %q", sv.ICAO24, "abc123")
	}
	if sv.Latitude != 0 {
		t.Errorf("Latitude = %f, want 0 for nil field", sv.Latitude)
	}
	if sv.OnGround != false {
		t.Errorf("OnGround = %v, want false for nil field", sv.OnGround)
	}
}

// TestGetStates_Success verifies that a valid API response is decoded into typed state vectors.
func TestGetStates_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"time": 1234567890,
			"states": [
				["abc123", "UAL123  ", "United States", 1234, 1234, -118.41, 33.94, 3048.0, false, 125.5, 270.0, -5.2, null, 3100.0, "1234", false, 0]
			]
		}`))
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), baseURL: srv.URL, clientID: "test", clientSecret: "secret"}
	resp, err := c.GetStates(context.Background(), geo.BBox{MinLat: 33, MaxLat: 35, MinLon: -119, MaxLon: -117})
	if err != nil {
		t.Fatalf("GetStates() error = %v", err)
	}
	if resp.Time != 1234567890 {
		t.Errorf("Time = %d, want 1234567890", resp.Time)
	}
	if len(resp.States) != 1 {
		t.Fatalf("len(States) = %d, want 1", len(resp.States))
	}
	if resp.States[0].ICAO24 != "abc123" {
		t.Errorf("States[0].ICAO24 = %q, want %q", resp.States[0].ICAO24, "abc123")
	}
}

// TestGetStates_ServerError verifies that a non-200 response returns an error.
func TestGetStates_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
	_, err := c.GetStates(context.Background(), geo.BBox{})
	if err == nil {
		t.Error("GetStates() expected error for 503 response, got nil")
	}
}

// TestGetStates_InvalidJSON verifies that a malformed response returns an error.
func TestGetStates_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
	_, err := c.GetStates(context.Background(), geo.BBox{})
	if err == nil {
		t.Error("GetStates() expected error for invalid JSON, got nil")
	}
}

// TestGetStates_EmptyStates verifies that a response with no aircraft returns an empty slice.
func TestGetStates_EmptyStates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"time": 1234, "states": null}`))
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
	resp, err := c.GetStates(context.Background(), geo.BBox{})
	if err != nil {
		t.Fatalf("GetStates() error = %v", err)
	}
	if len(resp.States) != 0 {
		t.Errorf("len(States) = %d, want 0", len(resp.States))
	}
}

// TestGetStates_SetsBasicAuth verifies that client credentials are sent as basic auth.
func TestGetStates_SetsBasicAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			t.Error("expected basic auth header")
		}
		if user != "my-client" || pass != "my-secret" {
			t.Errorf("auth = %q:%q, want my-client:my-secret", user, pass)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"time": 1234, "states": null}`))
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), baseURL: srv.URL, clientID: "my-client", clientSecret: "my-secret"}
	_, err := c.GetStates(context.Background(), geo.BBox{})
	if err != nil {
		t.Fatalf("GetStates() error = %v", err)
	}
}
