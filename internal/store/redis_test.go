// -------------------------------------------------------------------------------
// Store - Redis Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Covers the serialization, key-layout, and liveness logic in RedisStore against
// an in-process miniredis rather than a container, so these are not skipped
// under -short.
//
// miniredis speaks the real protocol, so the go-redis client, the SCAN
// iterator, and the EXISTS pipelines all execute for real; only the server is
// substituted. That is what makes the defensive paths reachable here — a key
// holding the wrong type, a value that is not valid JSON — which are the cases
// a live database makes awkward to arrange. The testcontainers suite in
// redis_integration_test.go remains the check against a real server.
// -------------------------------------------------------------------------------

package store

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/afreidah/flight-fetcher/internal/apiclient/opensky"

	"github.com/alicebob/miniredis/v2"
)

// -------------------------------------------------------------------------
// FIXTURES
// -------------------------------------------------------------------------

// newMiniredisStore starts an in-process Redis and returns a store pointed at
// it alongside the server handle, which tests use to seed raw values and to
// advance the clock for TTL assertions. Both are torn down with the test.
func newMiniredisStore(t *testing.T, ttl time.Duration) (*RedisStore, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	store := NewRedisStore(mr.Addr(), "", 0, ttl)
	t.Cleanup(func() { _ = store.Close() })
	return store, mr
}

// mustSet seeds a raw string value, failing the test if the server rejects it.
// Used to plant values the store itself would never write, which is how the
// defensive decode paths are reached.
func mustSet(t *testing.T, mr *miniredis.Miniredis, key, val string) {
	t.Helper()
	if err := mr.Set(key, val); err != nil {
		t.Fatalf("seeding %q: %v", key, err)
	}
}

// -------------------------------------------------------------------------
// FLIGHT STATE
// -------------------------------------------------------------------------

// TestSetFlight_RoundTrip verifies a state vector survives the JSON encode,
// the write, and the decode unchanged, and that it is stored under the
// prefixed key rather than the bare ICAO24.
func TestSetFlight_RoundTrip(t *testing.T) {
	store, mr := newMiniredisStore(t, time.Minute)
	ctx := context.Background()

	geoAlt := 10670.5
	// The antenna-enriched fields are pointers and a slice, so they are the
	// ones that can silently round-trip as nil if a json tag or omitempty is
	// wrong. An all-scalar fixture would not catch that.
	sv := &opensky.StateVector{
		ICAO24:       "a1b2c3",
		Callsign:     "SWA123",
		Latitude:     34.05,
		Longitude:    -118.25,
		BaroAltitude: 10668,
		Velocity:     230.5,
		Heading:      95.5,
		OnGround:     false,
		Squawk:       "1200",
		GeoAltitude:  &geoAlt,
		NavModes:     []string{"autopilot", "vnav"},
	}
	if err := store.SetFlight(ctx, sv); err != nil {
		t.Fatalf("SetFlight() error = %v", err)
	}

	if !mr.Exists(flightKeyPrefix + "a1b2c3") {
		t.Fatalf("key %q not written", flightKeyPrefix+"a1b2c3")
	}

	got, err := store.GetFlight(ctx, "a1b2c3")
	if err != nil {
		t.Fatalf("GetFlight() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetFlight() = nil, want the stored vector")
	}
	if !reflect.DeepEqual(got, sv) {
		t.Errorf("GetFlight() = %+v, want %+v", *got, *sv)
	}
}

// TestSetFlight_AppliesTTL verifies the store's configured TTL is attached to
// the key, which is what makes an aircraft disappear once it stops being
// heard. The exact value matters: it is derived from the slowest poll interval
// in the entrypoint, so a dropped TTL would silently pin stale aircraft on the
// dashboard forever.
func TestSetFlight_AppliesTTL(t *testing.T) {
	store, mr := newMiniredisStore(t, 90*time.Second)
	ctx := context.Background()

	sv := &opensky.StateVector{ICAO24: "a1b2c3"}
	if err := store.SetFlight(ctx, sv); err != nil {
		t.Fatalf("SetFlight() error = %v", err)
	}

	if got := mr.TTL(flightKeyPrefix + "a1b2c3"); got != 90*time.Second {
		t.Errorf("TTL = %v, want %v", got, 90*time.Second)
	}

	mr.FastForward(91 * time.Second)
	got, err := store.GetFlight(ctx, "a1b2c3")
	if err != nil {
		t.Fatalf("GetFlight() after expiry error = %v", err)
	}
	if got != nil {
		t.Errorf("GetFlight() after expiry = %+v, want nil", got)
	}
}

// TestGetFlight_Missing verifies redis.Nil is translated to a nil result with
// no error, so a caller can tell "not tracked" from "lookup failed".
func TestGetFlight_Missing(t *testing.T) {
	store, _ := newMiniredisStore(t, time.Minute)

	got, err := store.GetFlight(context.Background(), "nosuch")
	if err != nil {
		t.Fatalf("GetFlight() error = %v, want nil", err)
	}
	if got != nil {
		t.Errorf("GetFlight() = %+v, want nil", got)
	}
}

// TestGetFlight_MalformedJSON verifies a corrupt value is reported as an error
// rather than returned as a zero-valued flight. Unlike the list path, a
// single-key lookup has no other entries to fall back on, so surfacing the
// failure is the honest answer.
func TestGetFlight_MalformedJSON(t *testing.T) {
	store, mr := newMiniredisStore(t, time.Minute)
	mustSet(t, mr, flightKeyPrefix+"a1b2c3", "{not json")

	got, err := store.GetFlight(context.Background(), "a1b2c3")
	if err == nil {
		t.Fatal("GetFlight() error = nil, want a decode error")
	}
	if got != nil {
		t.Errorf("GetFlight() = %+v, want nil", got)
	}
}

// TestGetAllFlights_NoKeys verifies an empty store yields an empty, non-nil
// slice. The dashboard marshals this directly, where nil would render as null
// instead of [].
func TestGetAllFlights_NoKeys(t *testing.T) {
	store, _ := newMiniredisStore(t, time.Minute)

	got, err := store.GetAllFlights(context.Background())
	if err != nil {
		t.Fatalf("GetAllFlights() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetAllFlights() = nil, want empty slice")
	}
	if len(got) != 0 {
		t.Errorf("GetAllFlights() returned %d flights, want 0", len(got))
	}
}

// TestGetAllFlights_ReturnsAll verifies every stored flight comes back and
// that unrelated keys sharing the database are not swept up by the scan.
func TestGetAllFlights_ReturnsAll(t *testing.T) {
	store, mr := newMiniredisStore(t, time.Minute)
	ctx := context.Background()

	for _, icao := range []string{"a1b2c3", "d4e5f6", "070809"} {
		if err := store.SetFlight(ctx, &opensky.StateVector{ICAO24: icao}); err != nil {
			t.Fatalf("SetFlight(%s) error = %v", icao, err)
		}
	}
	// A liveness marker and an unrelated key: neither carries the flight
	// prefix, so neither should appear in the result.
	mustSet(t, mr, heardKeyPrefix+"opensky:a1b2c3", "1")
	mustSet(t, mr, "unrelated", "value")

	got, err := store.GetAllFlights(ctx)
	if err != nil {
		t.Fatalf("GetAllFlights() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("GetAllFlights() returned %d flights, want 3", len(got))
	}

	seen := make(map[string]bool, len(got))
	for _, sv := range got {
		seen[sv.ICAO24] = true
	}
	for _, icao := range []string{"a1b2c3", "d4e5f6", "070809"} {
		if !seen[icao] {
			t.Errorf("flight %q missing from result", icao)
		}
	}
}

// TestGetAllFlights_SkipsUnreadable verifies the list endpoint degrades rather
// than fails: one bad entry is logged and dropped while the healthy flights
// still return. Both defensive branches are covered — a value that is not
// valid JSON, and a key holding a non-string type, which MGET reports as nil.
func TestGetAllFlights_SkipsUnreadable(t *testing.T) {
	store, mr := newMiniredisStore(t, time.Minute)
	ctx := context.Background()

	if err := store.SetFlight(ctx, &opensky.StateVector{ICAO24: "good01"}); err != nil {
		t.Fatalf("SetFlight() error = %v", err)
	}
	mustSet(t, mr, flightKeyPrefix+"bad001", "{not json")
	if _, err := mr.Lpush(flightKeyPrefix+"bad002", "wrong-type"); err != nil {
		t.Fatalf("seeding list key: %v", err)
	}

	got, err := store.GetAllFlights(ctx)
	if err != nil {
		t.Fatalf("GetAllFlights() error = %v, want the bad entries skipped", err)
	}
	if len(got) != 1 {
		t.Fatalf("GetAllFlights() returned %d flights, want 1", len(got))
	}
	if got[0].ICAO24 != "good01" {
		t.Errorf("surviving flight = %q, want %q", got[0].ICAO24, "good01")
	}
}

// -------------------------------------------------------------------------
// LIVENESS MARKERS
// -------------------------------------------------------------------------

// TestMarkHeard_AndHeardBy_Subset verifies HeardBy reports only the sources
// with a live marker, and preserves the caller's source order rather than
// Redis key order — the dashboard renders the result directly.
func TestMarkHeard_AndHeardBy_Subset(t *testing.T) {
	store, _ := newMiniredisStore(t, time.Minute)
	ctx := context.Background()

	if err := store.MarkHeard(ctx, "opensky", "a1b2c3", time.Minute); err != nil {
		t.Fatalf("MarkHeard(opensky) error = %v", err)
	}
	if err := store.MarkHeard(ctx, "antenna", "a1b2c3", time.Minute); err != nil {
		t.Fatalf("MarkHeard(antenna) error = %v", err)
	}

	got, err := store.HeardBy(ctx, "a1b2c3", []string{"opensky", "satellite", "antenna"})
	if err != nil {
		t.Fatalf("HeardBy() error = %v", err)
	}
	want := []string{"opensky", "antenna"}
	if len(got) != len(want) {
		t.Fatalf("HeardBy() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("HeardBy()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestHeardBy_NoSources verifies the empty-source short circuit, which keeps
// the store from opening a pipeline with nothing in it.
func TestHeardBy_NoSources(t *testing.T) {
	store, _ := newMiniredisStore(t, time.Minute)

	got, err := store.HeardBy(context.Background(), "a1b2c3", nil)
	if err != nil {
		t.Fatalf("HeardBy() error = %v", err)
	}
	if got != nil {
		t.Errorf("HeardBy() = %v, want nil", got)
	}
}

// TestMarkHeard_MarkerExpires verifies liveness decays on its own: the marker
// carries the TTL passed at write time, so a source that stops hearing an
// aircraft drops off without anything having to delete the key.
func TestMarkHeard_MarkerExpires(t *testing.T) {
	store, mr := newMiniredisStore(t, time.Minute)
	ctx := context.Background()

	if err := store.MarkHeard(ctx, "opensky", "a1b2c3", 30*time.Second); err != nil {
		t.Fatalf("MarkHeard() error = %v", err)
	}
	if got := mr.TTL(heardKeyPrefix + "opensky:a1b2c3"); got != 30*time.Second {
		t.Errorf("marker TTL = %v, want %v", got, 30*time.Second)
	}

	mr.FastForward(31 * time.Second)

	got, err := store.HeardBy(ctx, "a1b2c3", []string{"opensky"})
	if err != nil {
		t.Fatalf("HeardBy() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("HeardBy() = %v, want no live sources", got)
	}
}

// TestHeardByAll_Matrix verifies the batched form builds the right per-aircraft
// answer across the full icao x source grid, and that an aircraft heard by
// nobody is omitted from the map rather than mapped to an empty slice.
func TestHeardByAll_Matrix(t *testing.T) {
	store, _ := newMiniredisStore(t, time.Minute)
	ctx := context.Background()

	if err := store.MarkHeard(ctx, "opensky", "a1b2c3", time.Minute); err != nil {
		t.Fatalf("MarkHeard() error = %v", err)
	}
	if err := store.MarkHeard(ctx, "antenna", "a1b2c3", time.Minute); err != nil {
		t.Fatalf("MarkHeard() error = %v", err)
	}
	if err := store.MarkHeard(ctx, "antenna", "d4e5f6", time.Minute); err != nil {
		t.Fatalf("MarkHeard() error = %v", err)
	}

	got, err := store.HeardByAll(ctx, []string{"a1b2c3", "d4e5f6", "070809"}, []string{"opensky", "antenna"})
	if err != nil {
		t.Fatalf("HeardByAll() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("HeardByAll() has %d entries, want 2 (070809 heard by nobody)", len(got))
	}
	if len(got["a1b2c3"]) != 2 {
		t.Errorf("a1b2c3 heard by %v, want both sources", got["a1b2c3"])
	}
	if len(got["d4e5f6"]) != 1 || got["d4e5f6"][0] != "antenna" {
		t.Errorf("d4e5f6 heard by %v, want [antenna]", got["d4e5f6"])
	}
	if _, ok := got["070809"]; ok {
		t.Errorf("070809 present in result, want it omitted")
	}
}

// TestHeardByAll_EmptyInput verifies both short circuits return an empty,
// non-nil map, since the server marshals the result straight to JSON.
func TestHeardByAll_EmptyInput(t *testing.T) {
	store, _ := newMiniredisStore(t, time.Minute)
	ctx := context.Background()

	tests := []struct {
		name    string
		icaos   []string
		sources []string
	}{
		{"no aircraft", nil, []string{"opensky"}},
		{"no sources", []string{"a1b2c3"}, nil},
		{"neither", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.HeardByAll(ctx, tt.icaos, tt.sources)
			if err != nil {
				t.Fatalf("HeardByAll() error = %v", err)
			}
			if got == nil {
				t.Fatal("HeardByAll() = nil, want empty map")
			}
			if len(got) != 0 {
				t.Errorf("HeardByAll() = %v, want empty map", got)
			}
		})
	}
}

// -------------------------------------------------------------------------
// CONNECTION
// -------------------------------------------------------------------------

// TestPing_Reachable verifies Ping succeeds against a live server and fails
// once it goes away, which is the signal the health endpoint reports.
func TestPing_Reachable(t *testing.T) {
	store, mr := newMiniredisStore(t, time.Minute)
	ctx := context.Background()

	if err := store.Ping(ctx); err != nil {
		t.Fatalf("Ping() error = %v, want nil", err)
	}

	mr.Close()
	if err := store.Ping(ctx); err == nil {
		t.Error("Ping() error = nil after server shutdown, want a connection error")
	}
}
