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

// TestMarkHeard_AndHeardBy verifies that per-source liveness keys are set
// with the given TTL and read back correctly for both present and absent
// sources.
func TestMarkHeard_AndHeardBy(t *testing.T) {
	store := newTestRedisStore(t, time.Minute)
	ctx := context.Background()

	if err := store.MarkHeard(ctx, "antenna", "abc123", time.Minute); err != nil {
		t.Fatalf("MarkHeard() error = %v", err)
	}
	if err := store.MarkHeard(ctx, "opensky", "abc123", time.Minute); err != nil {
		t.Fatalf("MarkHeard() error = %v", err)
	}
	// Aircraft heard only by opensky:
	if err := store.MarkHeard(ctx, "opensky", "def456", time.Minute); err != nil {
		t.Fatalf("MarkHeard() error = %v", err)
	}

	// Both sources present for abc123.
	heard, err := store.HeardBy(ctx, "abc123", []string{"antenna", "opensky"})
	if err != nil {
		t.Fatalf("HeardBy() error = %v", err)
	}
	if len(heard) != 2 || heard[0] != "antenna" || heard[1] != "opensky" {
		t.Errorf("HeardBy(abc123) = %v, want [antenna opensky]", heard)
	}

	// Only opensky for def456.
	heard, _ = store.HeardBy(ctx, "def456", []string{"antenna", "opensky"})
	if len(heard) != 1 || heard[0] != "opensky" {
		t.Errorf("HeardBy(def456) = %v, want [opensky]", heard)
	}

	// Unknown ICAO: empty result, no error.
	heard, err = store.HeardBy(ctx, "ghi789", []string{"antenna", "opensky"})
	if err != nil {
		t.Fatalf("HeardBy() error = %v", err)
	}
	if len(heard) != 0 {
		t.Errorf("HeardBy(ghi789) = %v, want empty", heard)
	}

	// Empty sources short-circuits.
	heard, err = store.HeardBy(ctx, "abc123", nil)
	if err != nil || heard != nil {
		t.Errorf("HeardBy(nil sources) = %v, %v; want nil, nil", heard, err)
	}
}

// TestMarkHeard_TTLExpires verifies that liveness keys expire according to
// the TTL passed to MarkHeard so stale sources drop out of HeardBy results.
func TestMarkHeard_TTLExpires(t *testing.T) {
	store := newTestRedisStore(t, time.Minute)
	ctx := context.Background()

	if err := store.MarkHeard(ctx, "antenna", "abc123", 50*time.Millisecond); err != nil {
		t.Fatalf("MarkHeard() error = %v", err)
	}
	// Immediately visible.
	heard, _ := store.HeardBy(ctx, "abc123", []string{"antenna"})
	if len(heard) != 1 {
		t.Fatalf("immediately after MarkHeard, got %v, want [antenna]", heard)
	}

	time.Sleep(150 * time.Millisecond)

	heard, _ = store.HeardBy(ctx, "abc123", []string{"antenna"})
	if len(heard) != 0 {
		t.Errorf("after TTL expiry, HeardBy() = %v, want empty", heard)
	}
}

// TestHeardByAll verifies the batched form returns a source map for every
// aircraft with at least one active source, and omits aircraft with none.
func TestHeardByAll(t *testing.T) {
	store := newTestRedisStore(t, time.Minute)
	ctx := context.Background()

	_ = store.MarkHeard(ctx, "antenna", "aaa", time.Minute)
	_ = store.MarkHeard(ctx, "opensky", "aaa", time.Minute)
	_ = store.MarkHeard(ctx, "antenna", "bbb", time.Minute)
	_ = store.MarkHeard(ctx, "opensky", "ccc", time.Minute)
	// "ddd" has nothing.

	got, err := store.HeardByAll(ctx, []string{"aaa", "bbb", "ccc", "ddd"}, []string{"antenna", "opensky"})
	if err != nil {
		t.Fatalf("HeardByAll() error = %v", err)
	}
	if len(got["aaa"]) != 2 {
		t.Errorf("aaa heard_by = %v, want 2 sources", got["aaa"])
	}
	if len(got["bbb"]) != 1 || got["bbb"][0] != "antenna" {
		t.Errorf("bbb heard_by = %v, want [antenna]", got["bbb"])
	}
	if len(got["ccc"]) != 1 || got["ccc"][0] != "opensky" {
		t.Errorf("ccc heard_by = %v, want [opensky]", got["ccc"])
	}
	if _, present := got["ddd"]; present {
		t.Errorf("ddd should be omitted from map, got %v", got["ddd"])
	}

	// Empty inputs short-circuit to empty map.
	empty, err := store.HeardByAll(ctx, nil, []string{"antenna"})
	if err != nil || len(empty) != 0 {
		t.Errorf("HeardByAll(nil icaos) = %v, %v", empty, err)
	}
	empty, err = store.HeardByAll(ctx, []string{"aaa"}, nil)
	if err != nil || len(empty) != 0 {
		t.Errorf("HeardByAll(nil sources) = %v, %v", empty, err)
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
