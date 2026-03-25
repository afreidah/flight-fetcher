// -------------------------------------------------------------------------------
// OpenSky - Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests the OpenSky API client and state vector JSON unmarshaling: valid arrays,
// null fields, short arrays, type mismatches, full HTTP responses, and error
// handling.
// -------------------------------------------------------------------------------

package opensky

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/afreidah/flight-fetcher/internal/apiclient"
	"github.com/afreidah/flight-fetcher/internal/geo"
)

// testClient creates a Client pointed at the given test server with no auth.
func testClient(srv *httptest.Server) *Client {
	return &Client{
		Client: apiclient.New(apiclient.Options{
			BaseURL:      srv.URL,
			MaxBodyBytes: 50 * 1024 * 1024,
		}),
	}
}

// TestUnmarshalStateVector_Valid verifies that a complete state vector array is decoded correctly.
func TestUnmarshalStateVector_Valid(t *testing.T) {
	raw := `["abc123", "UAL123  ", "United States", 1234, 1234, -118.4081, 33.9425, 3048.0, false, 125.5, 270.0, -5.2, null, 3100.0, "1234", false, 0]`

	var sv StateVector
	if err := json.Unmarshal([]byte(raw), &sv); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
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
	if sv.Squawk != "1234" {
		t.Errorf("Squawk = %q, want %q", sv.Squawk, "1234")
	}
}

// TestUnmarshalStateVector_TooShort verifies that an undersized array returns an error.
func TestUnmarshalStateVector_TooShort(t *testing.T) {
	raw := `["abc123", "UAL123"]`
	var sv StateVector
	if err := json.Unmarshal([]byte(raw), &sv); err == nil {
		t.Error("Unmarshal() expected error for short array, got nil")
	}
}

// TestUnmarshalStateVector_NullFields verifies that null fields decode to zero values.
func TestUnmarshalStateVector_NullFields(t *testing.T) {
	raw := `["abc123", null, "United States", null, null, null, null, null, null, null, null, null, null, null, null]`

	var sv StateVector
	if err := json.Unmarshal([]byte(raw), &sv); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if sv.ICAO24 != "abc123" {
		t.Errorf("ICAO24 = %q, want %q", sv.ICAO24, "abc123")
	}
	if sv.Callsign != "" {
		t.Errorf("Callsign = %q, want empty for null field", sv.Callsign)
	}
	if sv.Latitude != 0 {
		t.Errorf("Latitude = %f, want 0 for null field", sv.Latitude)
	}
	if sv.OnGround != false {
		t.Errorf("OnGround = %v, want false for null field", sv.OnGround)
	}
}

// TestUnmarshalStateVector_TypeMismatch verifies that wrong types are treated as zero values.
func TestUnmarshalStateVector_TypeMismatch(t *testing.T) {
	raw := `["abc123", 12345, "United States", null, null, "not_a_number", true, null, null, null, null, null, null, null, null]`

	var sv StateVector
	if err := json.Unmarshal([]byte(raw), &sv); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if sv.Callsign != "" {
		t.Errorf("Callsign = %q, want empty for type mismatch", sv.Callsign)
	}
	if sv.Longitude != 0 {
		t.Errorf("Longitude = %f, want 0 for type mismatch", sv.Longitude)
	}
}

// TestUnmarshalStateVector_InvalidJSON verifies that malformed JSON returns an error.
func TestUnmarshalStateVector_InvalidJSON(t *testing.T) {
	raw := `not json at all`
	var sv StateVector
	if err := json.Unmarshal([]byte(raw), &sv); err == nil {
		t.Error("Unmarshal() expected error for invalid JSON, got nil")
	}
}

// TestUnmarshalStateVector_ObjectFormat verifies that standard JSON object format is decoded correctly.
func TestUnmarshalStateVector_ObjectFormat(t *testing.T) {
	raw := `{"icao24":"abc123","callsign":"UAL123","origin_country":"United States","longitude":-118.41,"latitude":33.94,"baro_altitude":3048,"velocity":125.5,"heading":270,"vertical_rate":-5.2,"on_ground":false}`
	var sv StateVector
	if err := json.Unmarshal([]byte(raw), &sv); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if sv.ICAO24 != "abc123" {
		t.Errorf("ICAO24 = %q, want %q", sv.ICAO24, "abc123")
	}
	if sv.Velocity != 125.5 {
		t.Errorf("Velocity = %f, want 125.5", sv.Velocity)
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

	c := testClient(srv)
	c.clientID = "test"
	c.clientSecret = "secret"
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

// TestGetStates_SkipsMalformed verifies that malformed state vectors are skipped while valid ones are kept.
func TestGetStates_SkipsMalformed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"time": 1234,
			"states": [
				["good1", "UAL1  ", "US", null, null, -118.0, 34.0, 3000.0, false, 100.0, 90.0, 0.0, null, null, "1200"],
				["short"],
				["good2", "UAL2  ", "US", null, null, -118.0, 34.0, 3000.0, false, 100.0, 90.0, 0.0, null, null, "7700"]
			]
		}`))
	}))
	defer srv.Close()

	c := testClient(srv)
	resp, err := c.GetStates(context.Background(), geo.BBox{})
	if err != nil {
		t.Fatalf("GetStates() error = %v", err)
	}
	if len(resp.States) != 2 {
		t.Fatalf("len(States) = %d, want 2 (malformed vector should be skipped)", len(resp.States))
	}
	if resp.States[0].ICAO24 != "good1" {
		t.Errorf("States[0].ICAO24 = %q, want %q", resp.States[0].ICAO24, "good1")
	}
	if resp.States[1].ICAO24 != "good2" {
		t.Errorf("States[1].ICAO24 = %q, want %q", resp.States[1].ICAO24, "good2")
	}
}

// TestGetStates_RateLimit verifies that a 429 response triggers backoff.
func TestGetStates_RateLimit(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"time": 1234, "states": null}`))
	}))
	defer srv.Close()

	c := testClient(srv)

	// First call gets 429
	_, err := c.GetStates(context.Background(), geo.BBox{})
	if err == nil {
		t.Fatal("GetStates() expected error for 429 response, got nil")
	}

	// Second call should be blocked by backoff without hitting the server
	_, err = c.GetStates(context.Background(), geo.BBox{})
	if err == nil {
		t.Fatal("GetStates() expected backoff error, got nil")
	}
	if calls != 1 {
		t.Errorf("server received %d calls, want 1 (second should be blocked by backoff)", calls)
	}
}

// TestGetStates_ServerError verifies that a non-200 response returns an error.
func TestGetStates_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := testClient(srv)
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

	c := testClient(srv)
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

	c := testClient(srv)
	resp, err := c.GetStates(context.Background(), geo.BBox{})
	if err != nil {
		t.Fatalf("GetStates() error = %v", err)
	}
	if len(resp.States) != 0 {
		t.Errorf("len(States) = %d, want 0", len(resp.States))
	}
}

// TestGetStates_SetsBearerToken verifies that an OAuth2 token is fetched and sent as a bearer token.
func TestGetStates_SetsBearerToken(t *testing.T) {
	mux := http.NewServeMux()

	// Token endpoint
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("grant_type") != "client_credentials" {
			t.Errorf("grant_type = %q, want client_credentials", r.FormValue("grant_type"))
		}
		if r.FormValue("client_id") != "my-client" {
			t.Errorf("client_id = %q, want my-client", r.FormValue("client_id"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token": "test-token-123", "expires_in": 300}`))
	})

	// API endpoint
	mux.HandleFunc("GET /states/all", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token-123" {
			t.Errorf("Authorization = %q, want Bearer test-token-123", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"time": 1234, "states": null}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := testClient(srv)
	c.clientID = "my-client"
	c.clientSecret = "my-secret"
	c.tokenURL = srv.URL + "/token"

	_, err := c.GetStates(context.Background(), geo.BBox{})
	if err != nil {
		t.Fatalf("GetStates() error = %v", err)
	}
}
