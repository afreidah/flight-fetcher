// -------------------------------------------------------------------------------
// Store - Redis Integration Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests RedisStore methods against a real Redis instance via testcontainers.
// Verifies flight state round-trips, TTL expiration, SCAN+MGET retrieval,
// and connection lifecycle. Skipped with -short flag.
// -------------------------------------------------------------------------------

package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/afreidah/flight-fetcher/internal/apiclient/opensky"
)

// -------------------------------------------------------------------------
// TEST HELPERS
// -------------------------------------------------------------------------

// newTestRedisStore creates a RedisStore connected to the test container.
func newTestRedisStore(t *testing.T, ttl time.Duration) *RedisStore {
	t.Helper()
	skipShort(t)

	// testRedisAddr from testcontainers returns "redis://host:port/0"
	// NewRedisStore expects "host:port"
	addr := strings.TrimPrefix(testRedisAddr, "redis://")
	addr = strings.TrimSuffix(addr, "/0")

	store := NewRedisStore(addr, "", 0, ttl)
	t.Cleanup(func() {
		store.client.FlushDB(context.Background())
		store.Close()
	})
	return store
}

// -------------------------------------------------------------------------
// TESTS
// -------------------------------------------------------------------------

// TestSetAndGetFlight verifies round-trip flight state storage.
func TestSetAndGetFlight(t *testing.T) {
	store := newTestRedisStore(t, time.Minute)
	ctx := context.Background()

	sv := &opensky.StateVector{
		ICAO24:   "abc123",
		Callsign: "UAL123",
		Latitude: 34.09,
		Longitude: -118.33,
		BaroAltitude: 3048.0,
		Velocity: 125.5,
		Heading:  270.0,
		OnGround: false,
		Squawk:   "1234",
	}
	if err := store.SetFlight(ctx, sv); err != nil {
		t.Fatalf("SetFlight() error = %v", err)
	}

	got, err := store.GetFlight(ctx, "abc123")
	if err != nil {
		t.Fatalf("GetFlight() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetFlight() returned nil")
	}
	if got.ICAO24 != "abc123" {
		t.Errorf("ICAO24 = %q, want %q", got.ICAO24, "abc123")
	}
	if got.Callsign != "UAL123" {
		t.Errorf("Callsign = %q, want %q", got.Callsign, "UAL123")
	}
	if got.Velocity != 125.5 {
		t.Errorf("Velocity = %f, want 125.5", got.Velocity)
	}
}

// TestGetFlight_NotFound verifies that a missing flight returns nil.
func TestGetFlight_NotFound(t *testing.T) {
	store := newTestRedisStore(t, time.Minute)

	got, err := store.GetFlight(context.Background(), "unknown")
	if err != nil {
		t.Fatalf("GetFlight() error = %v", err)
	}
	if got != nil {
		t.Errorf("GetFlight() = %v, want nil", got)
	}
}

// TestGetAllFlights verifies that multiple flights are returned via SCAN+MGET.
func TestGetAllFlights(t *testing.T) {
	store := newTestRedisStore(t, time.Minute)
	ctx := context.Background()

	flights := []opensky.StateVector{
		{ICAO24: "aaa111", Callsign: "UAL1", Latitude: 34.0, Longitude: -118.0},
		{ICAO24: "bbb222", Callsign: "DAL2", Latitude: 35.0, Longitude: -119.0},
		{ICAO24: "ccc333", Callsign: "SWA3", Latitude: 36.0, Longitude: -120.0},
	}
	for i := range flights {
		must(t, store.SetFlight(ctx, &flights[i]))
	}

	got, err := store.GetAllFlights(ctx)
	if err != nil {
		t.Fatalf("GetAllFlights() error = %v", err)
	}
	if len(got) != 3 {
		t.Errorf("len(flights) = %d, want 3", len(got))
	}
}

// TestGetAllFlights_Empty verifies that no flights returns an empty slice.
func TestGetAllFlights_Empty(t *testing.T) {
	store := newTestRedisStore(t, time.Minute)

	got, err := store.GetAllFlights(context.Background())
	if err != nil {
		t.Fatalf("GetAllFlights() error = %v", err)
	}
	if got == nil {
		t.Error("GetAllFlights() returned nil, want empty slice")
	}
	if len(got) != 0 {
		t.Errorf("len(flights) = %d, want 0", len(got))
	}
}

// TestSetFlight_TTLExpiry verifies that flights expire after the configured TTL.
func TestSetFlight_TTLExpiry(t *testing.T) {
	store := newTestRedisStore(t, 50*time.Millisecond)
	ctx := context.Background()

	must(t, store.SetFlight(ctx, &opensky.StateVector{ICAO24: "expire"}))
	time.Sleep(100 * time.Millisecond)

	got, err := store.GetFlight(ctx, "expire")
	if err != nil {
		t.Fatalf("GetFlight() error = %v", err)
	}
	if got != nil {
		t.Error("GetFlight() should return nil after TTL expiry")
	}
}

// TestSetFlight_Overwrite verifies that a second set overwrites the first.
func TestSetFlight_Overwrite(t *testing.T) {
	store := newTestRedisStore(t, time.Minute)
	ctx := context.Background()

	must(t, store.SetFlight(ctx, &opensky.StateVector{ICAO24: "abc123", Callsign: "OLD"}))
	must(t, store.SetFlight(ctx, &opensky.StateVector{ICAO24: "abc123", Callsign: "NEW"}))

	got, _ := store.GetFlight(ctx, "abc123")
	if got.Callsign != "NEW" {
		t.Errorf("Callsign = %q, want %q after overwrite", got.Callsign, "NEW")
	}
}

// TestPing_Redis verifies that Ping succeeds on a healthy connection.
func TestPing_Redis(t *testing.T) {
	store := newTestRedisStore(t, time.Minute)

	if err := store.Ping(context.Background()); err != nil {
		t.Errorf("Ping() error = %v", err)
	}
}

// TestClose_Redis verifies that Ping fails after Close.
func TestClose_Redis(t *testing.T) {
	skipShort(t)

	addr := strings.TrimPrefix(testRedisAddr, "redis://")
	addr = strings.TrimSuffix(addr, "/0")
	store := NewRedisStore(addr, "", 0, time.Minute)

	store.Close()

	if err := store.Ping(context.Background()); err == nil {
		t.Error("Ping() should fail after Close()")
	}
}
