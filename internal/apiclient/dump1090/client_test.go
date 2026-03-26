// -------------------------------------------------------------------------------
// dump1090 - Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests the dump1090 client: feed parsing, bounding box filtering, unit
// conversion, missing position handling, and error cases.
// -------------------------------------------------------------------------------

package dump1090

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/afreidah/flight-fetcher/internal/apiclient"
	"github.com/afreidah/flight-fetcher/internal/geo"
)

func testClient(srv *httptest.Server) *Client {
	return &Client{Client: apiclient.New(apiclient.Options{BaseURL: srv.URL})}
}

// TestNewClient verifies the constructor.
func TestNewClient(t *testing.T) {
	c := NewClient("http://piaware:8080/")
	if c == nil {
		t.Fatal("NewClient() returned nil")
	}
}

// TestGetStates_FetchError verifies GetStates returns error when feed fails.
func TestGetStates_FetchError(t *testing.T) {
	c := testClient(httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))
	c.Client = apiclient.New(apiclient.Options{BaseURL: "http://127.0.0.1:1"})

	_, err := c.GetStates(context.Background(), geo.BBox{MinLat: -90, MaxLat: 90, MinLon: -180, MaxLon: 180})
	if err == nil {
		t.Fatal("GetStates() expected error, got nil")
	}
}

const sampleFeed = `{
	"now": 1711411200.0,
	"messages": 12345,
	"aircraft": [
		{
			"hex": "a835af",
			"flight": "UAL123  ",
			"lat": 34.09,
			"lon": -118.33,
			"alt_baro": 35000,
			"alt_geom": 35200,
			"gs": 450.5,
			"ias": 280,
			"tas": 440,
			"mach": 0.78,
			"track": 270.0,
			"track_rate": -0.1,
			"mag_heading": 268.5,
			"true_heading": 270.2,
			"roll": -1.5,
			"baro_rate": -500,
			"geom_rate": -480,
			"squawk": "1200",
			"category": "A3",
			"nav_qnh": 1013.25,
			"nav_altitude_mcp": 35000,
			"nav_altitude_fms": 35000,
			"nav_heading": 270,
			"nav_modes": ["autopilot", "althold", "lnav"],
			"nic": 8,
			"rc": 186,
			"nic_baro": 1,
			"nac_p": 10,
			"nac_v": 2,
			"sil": 3,
			"sil_type": "perhour",
			"version": 2,
			"wd": 240,
			"ws": 35,
			"oat": -45.5,
			"tat": -38.2,
			"messages": 5432,
			"seen": 0.3,
			"rssi": -8.5,
			"ground": false,
			"r": "N628TS",
			"t": "G650",
			"dbFlags": 0
		},
		{
			"hex": "ae1234",
			"flight": "RCH501  ",
			"lat": 34.10,
			"lon": -118.40,
			"alt_baro": 28000,
			"gs": 380,
			"track": 180,
			"squawk": "4512",
			"ground": false,
			"dbFlags": 1
		},
		{
			"hex": "abcdef",
			"flight": "",
			"messages": 10,
			"seen": 5.0
		}
	]
}`

// TestFetch_Success verifies full feed parsing with all fields.
func TestFetch_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/data/aircraft.json" {
			t.Errorf("path = %s, want /data/aircraft.json", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleFeed))
	}))
	defer srv.Close()

	c := testClient(srv)
	feed, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if len(feed.Aircraft) != 3 {
		t.Fatalf("aircraft count = %d, want 3", len(feed.Aircraft))
	}

	a := feed.Aircraft[0]
	if a.Hex != "a835af" {
		t.Errorf("hex = %q, want %q", a.Hex, "a835af")
	}
	if a.Flight != "UAL123  " {
		t.Errorf("flight = %q, want %q", a.Flight, "UAL123  ")
	}
	if a.Registration != "N628TS" {
		t.Errorf("registration = %q, want %q", a.Registration, "N628TS")
	}
	if a.AircraftType != "G650" {
		t.Errorf("aircraft type = %q, want %q", a.AircraftType, "G650")
	}
	if a.IsMilitary() {
		t.Error("aircraft should not be military")
	}
	if a.Mach == nil || *a.Mach != 0.78 {
		t.Errorf("mach = %v, want 0.78", a.Mach)
	}
	if a.OAT == nil || *a.OAT != -45.5 {
		t.Errorf("oat = %v, want -45.5", a.OAT)
	}
	if len(a.NavModes) != 3 {
		t.Errorf("nav_modes count = %d, want 3", len(a.NavModes))
	}

	mil := feed.Aircraft[1]
	if !mil.IsMilitary() {
		t.Error("second aircraft should be military (dbFlags=1)")
	}
}

// TestGetStates_FiltersByBBox verifies bounding box filtering and unit conversion.
func TestGetStates_FiltersByBBox(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleFeed))
	}))
	defer srv.Close()

	c := testClient(srv)
	bbox := geo.BBox{MinLat: 34.0, MaxLat: 34.15, MinLon: -118.5, MaxLon: -118.0}
	result, err := c.GetStates(context.Background(), bbox)
	if err != nil {
		t.Fatalf("GetStates() error = %v", err)
	}

	// Two aircraft have positions in bbox, one has no position
	if len(result.States) != 2 {
		t.Fatalf("states count = %d, want 2", len(result.States))
	}

	sv := result.States[0]
	if sv.ICAO24 != "a835af" {
		t.Errorf("icao24 = %q, want %q", sv.ICAO24, "a835af")
	}
	if sv.Callsign != "UAL123" {
		t.Errorf("callsign = %q, want %q (trimmed)", sv.Callsign, "UAL123")
	}
	// 35000 ft * 0.3048 = 10668 m
	if sv.BaroAltitude < 10667 || sv.BaroAltitude > 10669 {
		t.Errorf("baro_altitude = %f, want ~10668 (35000ft in meters)", sv.BaroAltitude)
	}
	// 450.5 knots * 0.514444 = ~231.8 m/s
	if sv.Velocity < 231 || sv.Velocity > 232 {
		t.Errorf("velocity = %f, want ~231.8 (450.5kt in m/s)", sv.Velocity)
	}
}

// TestGetStates_NoPosition verifies aircraft without position are excluded.
func TestGetStates_NoPosition(t *testing.T) {
	feed := `{"now":1234,"aircraft":[{"hex":"abc123","messages":10}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(feed))
	}))
	defer srv.Close()

	c := testClient(srv)
	result, err := c.GetStates(context.Background(), geo.BBox{MinLat: -90, MaxLat: 90, MinLon: -180, MaxLon: 180})
	if err != nil {
		t.Fatalf("GetStates() error = %v", err)
	}
	if len(result.States) != 0 {
		t.Errorf("states count = %d, want 0 (no position)", len(result.States))
	}
}

// TestGetStates_OutsideBBox verifies aircraft outside the bbox are filtered.
func TestGetStates_OutsideBBox(t *testing.T) {
	feed := `{"now":1234,"aircraft":[{"hex":"abc123","lat":50.0,"lon":10.0}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(feed))
	}))
	defer srv.Close()

	c := testClient(srv)
	bbox := geo.BBox{MinLat: 34.0, MaxLat: 35.0, MinLon: -119.0, MaxLon: -117.0}
	result, err := c.GetStates(context.Background(), bbox)
	if err != nil {
		t.Fatalf("GetStates() error = %v", err)
	}
	if len(result.States) != 0 {
		t.Errorf("states count = %d, want 0 (outside bbox)", len(result.States))
	}
}

// TestFetch_ServerError verifies error handling for non-200 responses.
func TestFetch_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := testClient(srv)
	_, err := c.Fetch(context.Background())
	if err == nil {
		t.Fatal("Fetch() expected error for 500 response, got nil")
	}
}

// TestFetch_InvalidJSON verifies error handling for malformed responses.
func TestFetch_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	c := testClient(srv)
	_, err := c.Fetch(context.Background())
	if err == nil {
		t.Fatal("Fetch() expected error for invalid JSON, got nil")
	}
}

// TestFetch_TransportError verifies error handling for connection failures.
func TestFetch_TransportError(t *testing.T) {
	c := testClient(httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))
	c.Client = apiclient.New(apiclient.Options{BaseURL: "http://127.0.0.1:1"})

	_, err := c.Fetch(context.Background())
	if err == nil {
		t.Fatal("Fetch() expected error for unreachable server, got nil")
	}
}

// TestToStateVector_NilFields verifies conversion with minimal data.
func TestToStateVector_NilFields(t *testing.T) {
	lat, lon := 34.09, -118.33
	a := Aircraft{
		Hex: "ABC123",
		Lat: &lat,
		Lon: &lon,
	}
	sv := a.ToStateVector()
	if sv == nil {
		t.Fatal("ToStateVector() = nil, want result")
	}
	if sv.ICAO24 != "abc123" {
		t.Errorf("icao24 = %q, want lowercase", sv.ICAO24)
	}
	if sv.BaroAltitude != 0 {
		t.Errorf("baro_altitude = %f, want 0 for nil", sv.BaroAltitude)
	}
}
