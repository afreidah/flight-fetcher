// -------------------------------------------------------------------------------
// Store - PostgreSQL Integration Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests PostgresStore methods against a real PostgreSQL instance via
// testcontainers. Verifies migrations, CRUD operations, upsert behavior,
// TTL-aware reads, and batched retention deletes. Skipped with -short flag.
// -------------------------------------------------------------------------------

package store

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/afreidah/flight-fetcher/internal/aircraft"
	"github.com/afreidah/flight-fetcher/internal/route"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	redisModule "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

// -------------------------------------------------------------------------
// TEST SETUP
// -------------------------------------------------------------------------

// testDSN holds the DSN for the test Postgres container.
var testDSN string

// testPool holds a direct connection pool for test assertions.
var testPool *pgxpool.Pool

// testRedisAddr holds the address for the test Redis container.
var testRedisAddr string

// TestMain starts Postgres and Redis containers, runs migrations, and
// executes all store integration tests.
func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		os.Exit(m.Run())
	}

	ctx := context.Background()

	// Start Postgres
	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("flight_fetcher_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start postgres container: %v\n", err)
		os.Exit(1)
	}

	testDSN, err = pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get postgres connection string: %v\n", err)
		os.Exit(1)
	}

	testPool, err = pgxpool.New(ctx, testDSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create test pool: %v\n", err)
		os.Exit(1)
	}

	// Run migrations via a throwaway store construction
	if _, err := NewPostgresStore(ctx, testDSN, 0); err != nil {
		fmt.Fprintf(os.Stderr, "failed to run migrations: %v\n", err)
		os.Exit(1)
	}

	// Start Redis
	redisContainer, err := redisModule.Run(ctx, "redis:7-alpine")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start redis container: %v\n", err)
		os.Exit(1)
	}

	testRedisAddr, err = redisContainer.ConnectionString(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get redis connection string: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	testPool.Close()
	_ = pgContainer.Terminate(ctx)
	_ = redisContainer.Terminate(ctx)
	os.Exit(code)
}

// -------------------------------------------------------------------------
// TEST HELPERS
// -------------------------------------------------------------------------

// skipShort skips the test if the -short flag is set.
func skipShort(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
}

// newTestStore creates a PostgresStore connected to the test container.
func newTestStore(t *testing.T, routeTTL time.Duration) *PostgresStore {
	t.Helper()
	skipShort(t)
	ctx := context.Background()
	store, err := NewPostgresStore(ctx, testDSN, routeTTL)
	if err != nil {
		t.Fatalf("NewPostgresStore() error = %v", err)
	}
	t.Cleanup(func() {
		truncateAll(t)
		store.Close()
	})
	return store
}

// must fails the test if err is non-nil.
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// truncateAll removes all rows from all tables.
func truncateAll(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(),
		"TRUNCATE aircraft_meta, sightings, flight_routes, squawk_alerts RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("truncateAll() error = %v", err)
	}
}

// -------------------------------------------------------------------------
// TESTS
// -------------------------------------------------------------------------

// TestNewPostgresStore_RunsMigrations verifies that all tables are created by migrations.
func TestNewPostgresStore_RunsMigrations(t *testing.T) {
	skipShort(t)

	tables := []string{"aircraft_meta", "sightings", "flight_routes", "squawk_alerts"}
	for _, table := range tables {
		var exists bool
		err := testPool.QueryRow(context.Background(),
			"SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name=$1)", table).Scan(&exists)
		if err != nil {
			t.Fatalf("checking table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("table %s does not exist after migrations", table)
		}
	}
}

// TestSaveAndGetAircraftMeta verifies round-trip aircraft metadata storage.
func TestSaveAndGetAircraftMeta(t *testing.T) {
	store := newTestStore(t, 0)
	ctx := context.Background()

	info := &aircraft.Info{
		ICAO24:           "abc123",
		Registration:     "N12345",
		ManufacturerName: "Boeing",
		Type:             "737-800",
		OperatorFlagCode: "UAL",
	}
	if err := store.SaveAircraftMeta(ctx, info); err != nil {
		t.Fatalf("SaveAircraftMeta() error = %v", err)
	}

	got, err := store.GetAircraftMeta(ctx, "abc123")
	if err != nil {
		t.Fatalf("GetAircraftMeta() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetAircraftMeta() returned nil")
	}
	if got.Registration != "N12345" {
		t.Errorf("Registration = %q, want %q", got.Registration, "N12345")
	}
	if got.ManufacturerName != "Boeing" {
		t.Errorf("ManufacturerName = %q, want %q", got.ManufacturerName, "Boeing")
	}
	if got.Type != "737-800" {
		t.Errorf("Type = %q, want %q", got.Type, "737-800")
	}
	if got.OperatorFlagCode != "UAL" {
		t.Errorf("OperatorFlagCode = %q, want %q", got.OperatorFlagCode, "UAL")
	}
}

// TestSaveAircraftMeta_Upsert verifies that a second save updates the existing row.
func TestSaveAircraftMeta_Upsert(t *testing.T) {
	store := newTestStore(t, 0)
	ctx := context.Background()

	must(t, store.SaveAircraftMeta(ctx, &aircraft.Info{ICAO24: "abc123", Registration: "N11111", Type: "A320"}))
	must(t, store.SaveAircraftMeta(ctx, &aircraft.Info{ICAO24: "abc123", Registration: "N22222", Type: "A321"}))

	got, _ := store.GetAircraftMeta(ctx, "abc123")
	if got.Registration != "N22222" {
		t.Errorf("Registration = %q, want %q after upsert", got.Registration, "N22222")
	}
	if got.Type != "A321" {
		t.Errorf("Type = %q, want %q after upsert", got.Type, "A321")
	}
}

// TestGetAircraftMeta_NotFound verifies that a missing ICAO24 returns nil without error.
func TestGetAircraftMeta_NotFound(t *testing.T) {
	store := newTestStore(t, 0)

	got, err := store.GetAircraftMeta(context.Background(), "unknown")
	if err != nil {
		t.Fatalf("GetAircraftMeta() error = %v", err)
	}
	if got != nil {
		t.Errorf("GetAircraftMeta() = %v, want nil", got)
	}
}

// TestLogSighting verifies that a sighting is persisted.
func TestLogSighting(t *testing.T) {
	store := newTestStore(t, 0)
	ctx := context.Background()

	if err := store.LogSighting(ctx, "abc123", 34.09, -118.33, 5.5); err != nil {
		t.Fatalf("LogSighting() error = %v", err)
	}

	var count int
	must(t, testPool.QueryRow(ctx, "SELECT COUNT(*) FROM sightings WHERE icao24='abc123'").Scan(&count))
	if count != 1 {
		t.Errorf("sightings count = %d, want 1", count)
	}
}

// TestSaveAndGetFlightRoute verifies round-trip route storage within TTL.
func TestSaveAndGetFlightRoute(t *testing.T) {
	store := newTestStore(t, time.Hour)
	ctx := context.Background()

	ri := &route.Info{
		FlightICAO: "AAL2079",
		DepIATA:    "LAX",
		DepICAO:    "KLAX",
		DepName:    "Los Angeles International",
		ArrIATA:    "DFW",
		ArrICAO:    "KDFW",
		ArrName:    "Dallas Fort Worth",
	}
	if err := store.SaveFlightRoute(ctx, ri); err != nil {
		t.Fatalf("SaveFlightRoute() error = %v", err)
	}

	got, err := store.GetFlightRoute(ctx, "AAL2079")
	if err != nil {
		t.Fatalf("GetFlightRoute() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetFlightRoute() returned nil")
	}
	if got.DepIATA != "LAX" {
		t.Errorf("DepIATA = %q, want %q", got.DepIATA, "LAX")
	}
	if got.ArrIATA != "DFW" {
		t.Errorf("ArrIATA = %q, want %q", got.ArrIATA, "DFW")
	}
}

// TestSaveFlightRoute_Upsert verifies that a second save updates the route.
func TestSaveFlightRoute_Upsert(t *testing.T) {
	store := newTestStore(t, time.Hour)
	ctx := context.Background()

	must(t, store.SaveFlightRoute(ctx, &route.Info{FlightICAO: "AAL100", DepIATA: "LAX", ArrIATA: "JFK"}))
	must(t, store.SaveFlightRoute(ctx, &route.Info{FlightICAO: "AAL100", DepIATA: "SFO", ArrIATA: "ORD"}))

	got, _ := store.GetFlightRoute(ctx, "AAL100")
	if got.DepIATA != "SFO" {
		t.Errorf("DepIATA = %q, want %q after upsert", got.DepIATA, "SFO")
	}
	if got.ArrIATA != "ORD" {
		t.Errorf("ArrIATA = %q, want %q after upsert", got.ArrIATA, "ORD")
	}
}

// TestGetFlightRoute_NotFound verifies that a missing callsign returns nil.
func TestGetFlightRoute_NotFound(t *testing.T) {
	store := newTestStore(t, time.Hour)

	got, err := store.GetFlightRoute(context.Background(), "UNKNOWN")
	if err != nil {
		t.Fatalf("GetFlightRoute() error = %v", err)
	}
	if got != nil {
		t.Errorf("GetFlightRoute() = %v, want nil", got)
	}
}

// TestGetFlightRoute_Stale verifies that an expired route returns nil.
func TestGetFlightRoute_Stale(t *testing.T) {
	store := newTestStore(t, time.Millisecond)
	ctx := context.Background()

	must(t, store.SaveFlightRoute(ctx, &route.Info{FlightICAO: "AAL100", DepIATA: "LAX", ArrIATA: "JFK"}))
	time.Sleep(10 * time.Millisecond)

	got, err := store.GetFlightRoute(ctx, "AAL100")
	if err != nil {
		t.Fatalf("GetFlightRoute() error = %v", err)
	}
	if got != nil {
		t.Errorf("GetFlightRoute() = %v, want nil for stale route", got)
	}
}

// TestInsertAndHasRecentSquawkAlert verifies alert insertion and cooldown check.
func TestInsertAndHasRecentSquawkAlert(t *testing.T) {
	store := newTestStore(t, 0)
	ctx := context.Background()

	if err := store.InsertSquawkAlert(ctx, "abc123", "UAL123", "7700", 34.09, -118.33); err != nil {
		t.Fatalf("InsertSquawkAlert() error = %v", err)
	}

	has, err := store.HasRecentSquawkAlert(ctx, "abc123", "7700", time.Hour)
	if err != nil {
		t.Fatalf("HasRecentSquawkAlert() error = %v", err)
	}
	if !has {
		t.Error("HasRecentSquawkAlert() = false, want true")
	}
}

// TestHasRecentSquawkAlert_NoneExists verifies false when no alert exists.
func TestHasRecentSquawkAlert_NoneExists(t *testing.T) {
	store := newTestStore(t, 0)

	has, err := store.HasRecentSquawkAlert(context.Background(), "unknown", "7700", time.Hour)
	if err != nil {
		t.Fatalf("HasRecentSquawkAlert() error = %v", err)
	}
	if has {
		t.Error("HasRecentSquawkAlert() = true, want false for nonexistent")
	}
}

// TestHasRecentSquawkAlert_DifferentSquawk verifies that a different squawk code is not matched.
func TestHasRecentSquawkAlert_DifferentSquawk(t *testing.T) {
	store := newTestStore(t, 0)
	ctx := context.Background()

	must(t, store.InsertSquawkAlert(ctx, "abc123", "UAL123", "7700", 34.09, -118.33))

	has, _ := store.HasRecentSquawkAlert(ctx, "abc123", "7600", time.Hour)
	if has {
		t.Error("HasRecentSquawkAlert() = true, want false for different squawk")
	}
}

// TestGetRecentSquawkAlerts verifies that alerts are returned ordered by seen_at DESC.
func TestGetRecentSquawkAlerts(t *testing.T) {
	store := newTestStore(t, 0)
	ctx := context.Background()

	must(t, store.InsertSquawkAlert(ctx, "first", "UAL1", "7700", 34.0, -118.0))
	time.Sleep(time.Millisecond)
	must(t, store.InsertSquawkAlert(ctx, "second", "UAL2", "7600", 35.0, -119.0))

	alerts, err := store.GetRecentSquawkAlerts(ctx, time.Hour)
	if err != nil {
		t.Fatalf("GetRecentSquawkAlerts() error = %v", err)
	}
	if len(alerts) != 2 {
		t.Fatalf("len(alerts) = %d, want 2", len(alerts))
	}
	if alerts[0].ICAO24 != "second" {
		t.Errorf("alerts[0].ICAO24 = %q, want %q (most recent first)", alerts[0].ICAO24, "second")
	}
	if alerts[1].ICAO24 != "first" {
		t.Errorf("alerts[1].ICAO24 = %q, want %q", alerts[1].ICAO24, "first")
	}
}

// TestGetRecentSquawkAlerts_Empty verifies that no alerts returns an empty slice.
func TestGetRecentSquawkAlerts_Empty(t *testing.T) {
	store := newTestStore(t, 0)

	alerts, err := store.GetRecentSquawkAlerts(context.Background(), time.Hour)
	if err != nil {
		t.Fatalf("GetRecentSquawkAlerts() error = %v", err)
	}
	if alerts == nil {
		t.Error("GetRecentSquawkAlerts() returned nil, want empty slice")
	}
	if len(alerts) != 0 {
		t.Errorf("len(alerts) = %d, want 0", len(alerts))
	}
}

// TestDeleteOldSightings verifies batched deletion of old sightings.
func TestDeleteOldSightings(t *testing.T) {
	store := newTestStore(t, 0)
	ctx := context.Background()

	// Insert old sightings via direct SQL to bypass time.Now()
	oldTime := time.Now().UTC().Add(-48 * time.Hour)
	for range 3 {
		_, err := testPool.Exec(ctx,
			"INSERT INTO sightings (icao24, lat, lon, distance_km, seen_at) VALUES ($1, $2, $3, $4, $5)",
			"old", 34.0, -118.0, 5.0, oldTime)
		must(t, err)
	}
	must(t, store.LogSighting(ctx, "recent", 34.0, -118.0, 5.0))

	deleted, err := store.DeleteOldSightings(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("DeleteOldSightings() error = %v", err)
	}
	if deleted != 3 {
		t.Errorf("deleted = %d, want 3", deleted)
	}

	var remaining int
	must(t, testPool.QueryRow(ctx, "SELECT COUNT(*) FROM sightings").Scan(&remaining))
	if remaining != 1 {
		t.Errorf("remaining sightings = %d, want 1", remaining)
	}
}

// TestDeleteOldSquawkAlerts verifies batched deletion of old alerts.
func TestDeleteOldSquawkAlerts(t *testing.T) {
	store := newTestStore(t, 0)
	ctx := context.Background()

	oldTime := time.Now().UTC().Add(-48 * time.Hour)
	_, err := testPool.Exec(ctx,
		"INSERT INTO squawk_alerts (icao24, callsign, squawk, lat, lon, seen_at) VALUES ($1, $2, $3, $4, $5, $6)",
		"old", "UAL1", "7700", 34.0, -118.0, oldTime)
	must(t, err)
	must(t, store.InsertSquawkAlert(ctx, "recent", "UAL2", "7700", 34.0, -118.0))

	deleted, err := store.DeleteOldSquawkAlerts(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("DeleteOldSquawkAlerts() error = %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}
}

// TestDeleteOldRoutes verifies batched deletion of stale routes.
func TestDeleteOldRoutes(t *testing.T) {
	store := newTestStore(t, 0)
	ctx := context.Background()

	oldTime := time.Now().UTC().Add(-48 * time.Hour)
	_, err := testPool.Exec(ctx,
		"INSERT INTO flight_routes (callsign, dep_iata, dep_icao, dep_name, arr_iata, arr_icao, arr_name, cached_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		"OLD100", "LAX", "KLAX", "LAX", "JFK", "KJFK", "JFK", oldTime)
	must(t, err)
	must(t, store.SaveFlightRoute(ctx, &route.Info{FlightICAO: "NEW100", DepIATA: "SFO", ArrIATA: "ORD"}))

	deleted, err := store.DeleteOldRoutes(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("DeleteOldRoutes() error = %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}
}

// TestDeleteOldSightings_NothingToDelete verifies zero count when nothing is old.
func TestDeleteOldSightings_NothingToDelete(t *testing.T) {
	store := newTestStore(t, 0)
	ctx := context.Background()

	must(t, store.LogSighting(ctx, "recent", 34.0, -118.0, 5.0))

	deleted, err := store.DeleteOldSightings(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("DeleteOldSightings() error = %v", err)
	}
	if deleted != 0 {
		t.Errorf("deleted = %d, want 0", deleted)
	}
}

// TestPing_Postgres verifies that Ping succeeds on a healthy connection.
func TestPing_Postgres(t *testing.T) {
	store := newTestStore(t, 0)

	if err := store.Ping(context.Background()); err != nil {
		t.Errorf("Ping() error = %v", err)
	}
}
