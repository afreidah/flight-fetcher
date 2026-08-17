// -------------------------------------------------------------------------------
// Store - PostgreSQL Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Covers the pure logic layered on top of the sqlc queries: mapping generated
// rows to the domain types, translating pgx.ErrNoRows into a nil result, and
// deriving every TTL, cooldown, and retention cutoff from the store's clock.
//
// These run against a fake querier and a frozen clock, so they need neither
// Docker nor a live database and are not skipped under -short. The
// testcontainers suites in postgres_integration_test.go remain the check on
// real SQL behaviour; what is here is the part that is awkward to provoke
// against a live database, where an exact cutoff instant or a specific driver
// error has to be observed rather than arranged.
// -------------------------------------------------------------------------------

package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/afreidah/flight-fetcher/internal/aircraft"
	"github.com/afreidah/flight-fetcher/internal/route"
	db "github.com/afreidah/flight-fetcher/internal/store/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel"
)

// -------------------------------------------------------------------------
// FIXTURES
// -------------------------------------------------------------------------

// testNow is the instant the fake clock is frozen at. Every cutoff assertion
// below is expressed relative to it, so a drift in the arithmetic shows up as
// an exact mismatch rather than a flake.
var testNow = time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)

// errQuery is the sentinel a fake query returns when the test wants to check
// that a driver failure reaches the caller unwrapped.
var errQuery = errors.New("query exploded")

// fakeQuerier implements querier. Each field is either the value to return or
// the error to fail with; the capture fields record what the store passed
// down, which is how the cutoff arithmetic is asserted.
type fakeQuerier struct {
	aircraftRow db.GetAircraftMetaRow
	aircraftErr error
	routeRow    db.GetFlightRouteRow
	routeErr    error
	alertRows   []db.SquawkAlert
	alertErr    error
	hasAlert    bool
	hasAlertErr error
	execErr     error
	tag         pgconn.CommandTag
	tagErr      error

	gotAircraft  db.GetAircraftMetaParams
	gotUpsert    db.UpsertAircraftMetaParams
	gotRoute     db.GetFlightRouteParams
	gotRouteSave db.UpsertFlightRouteParams
	gotSighting  db.LogSightingParams
	gotHasAlert  db.HasRecentSquawkAlertParams
	gotInsert    db.InsertSquawkAlertParams
	gotSince     pgtype.Timestamptz
	gotCutoff    pgtype.Timestamptz
}

// The three Upsert/Insert params below are passed by value because that is the
// signature sqlc generates and querier mirrors; hugeParam cannot be satisfied
// without diverging from the interface these fakes exist to implement.

//nolint:gocritic // signature fixed by the generated querier interface
func (f *fakeQuerier) UpsertAircraftMeta(_ context.Context, arg db.UpsertAircraftMetaParams) error {
	f.gotUpsert = arg
	return f.execErr
}

func (f *fakeQuerier) GetAircraftMeta(_ context.Context, arg db.GetAircraftMetaParams) (db.GetAircraftMetaRow, error) {
	f.gotAircraft = arg
	return f.aircraftRow, f.aircraftErr
}

func (f *fakeQuerier) LogSighting(_ context.Context, arg db.LogSightingParams) error {
	f.gotSighting = arg
	return f.execErr
}

//nolint:gocritic // signature fixed by the generated querier interface
func (f *fakeQuerier) UpsertFlightRoute(_ context.Context, arg db.UpsertFlightRouteParams) error {
	f.gotRouteSave = arg
	return f.execErr
}

func (f *fakeQuerier) GetFlightRoute(_ context.Context, arg db.GetFlightRouteParams) (db.GetFlightRouteRow, error) {
	f.gotRoute = arg
	return f.routeRow, f.routeErr
}

func (f *fakeQuerier) HasRecentSquawkAlert(_ context.Context, arg db.HasRecentSquawkAlertParams) (bool, error) {
	f.gotHasAlert = arg
	return f.hasAlert, f.hasAlertErr
}

//nolint:gocritic // signature fixed by the generated querier interface
func (f *fakeQuerier) InsertSquawkAlert(_ context.Context, arg db.InsertSquawkAlertParams) error {
	f.gotInsert = arg
	return f.execErr
}

func (f *fakeQuerier) GetRecentSquawkAlerts(_ context.Context, seenAt pgtype.Timestamptz) ([]db.SquawkAlert, error) {
	f.gotSince = seenAt
	return f.alertRows, f.alertErr
}

func (f *fakeQuerier) DeleteOldSightings(_ context.Context, seenAt pgtype.Timestamptz) (pgconn.CommandTag, error) {
	f.gotCutoff = seenAt
	return f.tag, f.tagErr
}

func (f *fakeQuerier) DeleteOldSquawkAlerts(_ context.Context, seenAt pgtype.Timestamptz) (pgconn.CommandTag, error) {
	f.gotCutoff = seenAt
	return f.tag, f.tagErr
}

func (f *fakeQuerier) DeleteOldRoutes(_ context.Context, cachedAt pgtype.Timestamptz) (pgconn.CommandTag, error) {
	f.gotCutoff = cachedAt
	return f.tag, f.tagErr
}

// newFakeStore builds a store around the fake with the clock frozen at
// testNow. pool stays nil: none of the methods under test touch it, and a nil
// pool means a test that starts doing real I/O fails loudly instead of quietly
// reaching a database.
func newFakeStore(q *fakeQuerier) *PostgresStore {
	return &PostgresStore{
		queries:     q,
		routeTTL:    DefaultRouteTTL,
		aircraftTTL: DefaultAircraftTTL,
		now:         func() time.Time { return testNow },
		tracer:      otel.Tracer("test"),
	}
}

// assertCutoff fails unless got is exactly testNow minus back.
func assertCutoff(t *testing.T, got pgtype.Timestamptz, back time.Duration) {
	t.Helper()
	want := testNow.Add(-back)
	if !got.Valid {
		t.Fatalf("cutoff not marked valid")
	}
	if !got.Time.Equal(want) {
		t.Errorf("cutoff = %v, want %v (testNow - %v)", got.Time, want, back)
	}
}

// -------------------------------------------------------------------------
// AIRCRAFT METADATA
// -------------------------------------------------------------------------

// TestGetAircraftMeta_Maps verifies every generated column reaches the right
// domain field. The two structs use different names on most fields, so a
// transposition here is silent at compile time.
func TestGetAircraftMeta_Maps(t *testing.T) {
	fake := &fakeQuerier{aircraftRow: db.GetAircraftMetaRow{
		Icao24:           "a1b2c3",
		Registration:     "N12345",
		Manufacturer:     "Boeing",
		Type:             "737-800",
		Operator:         "SWA",
		IcaoTypeCode:     "B738",
		RegisteredOwners: "Southwest Airlines",
		ImageUrl:         "https://img.example/a1b2c3.jpg",
	}}

	got, err := newFakeStore(fake).GetAircraftMeta(context.Background(), "a1b2c3")
	if err != nil {
		t.Fatalf("GetAircraftMeta() error = %v", err)
	}

	want := &aircraft.Info{
		ICAO24:           "a1b2c3",
		Registration:     "N12345",
		ManufacturerName: "Boeing",
		Type:             "737-800",
		OperatorFlagCode: "SWA",
		ICAOTypeCode:     "B738",
		RegisteredOwners: "Southwest Airlines",
		ImageURL:         "https://img.example/a1b2c3.jpg",
	}
	if *got != *want {
		t.Errorf("GetAircraftMeta() = %+v, want %+v", *got, *want)
	}

	if fake.gotAircraft.Icao24 != "a1b2c3" {
		t.Errorf("queried icao24 = %q, want %q", fake.gotAircraft.Icao24, "a1b2c3")
	}
	assertCutoff(t, fake.gotAircraft.UpdatedAt, DefaultAircraftTTL)
}

// TestGetAircraftMeta_Errors covers the two failure shapes: a miss (or an
// entry older than the TTL, which the query reports the same way) becomes a
// nil result with no error, while any other driver error reaches the caller.
func TestGetAircraftMeta_Errors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantErr error
	}{
		{"no rows is a miss, not an error", pgx.ErrNoRows, nil},
		{"driver error propagates", errQuery, errQuery},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newFakeStore(&fakeQuerier{aircraftErr: tt.err}).
				GetAircraftMeta(context.Background(), "a1b2c3")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if got != nil {
				t.Errorf("result = %+v, want nil", got)
			}
		})
	}
}

// TestSaveAircraftMeta_Maps verifies the domain type is unpacked into the
// upsert params, and that a write failure is returned rather than swallowed.
func TestSaveAircraftMeta_Maps(t *testing.T) {
	fake := &fakeQuerier{}
	info := &aircraft.Info{
		ICAO24:           "a1b2c3",
		Registration:     "N12345",
		ManufacturerName: "Airbus",
		Type:             "A320",
		OperatorFlagCode: "AAL",
		ICAOTypeCode:     "A320",
		RegisteredOwners: "American Airlines",
		ImageURL:         "https://img.example/x.jpg",
	}
	if err := newFakeStore(fake).SaveAircraftMeta(context.Background(), info); err != nil {
		t.Fatalf("SaveAircraftMeta() error = %v", err)
	}

	want := db.UpsertAircraftMetaParams{
		Icao24:           "a1b2c3",
		Registration:     "N12345",
		Manufacturer:     "Airbus",
		Type:             "A320",
		Operator:         "AAL",
		IcaoTypeCode:     "A320",
		RegisteredOwners: "American Airlines",
		ImageUrl:         "https://img.example/x.jpg",
	}
	if fake.gotUpsert != want {
		t.Errorf("upsert params = %+v, want %+v", fake.gotUpsert, want)
	}

	failing := &fakeQuerier{execErr: errQuery}
	if err := newFakeStore(failing).SaveAircraftMeta(context.Background(), info); !errors.Is(err, errQuery) {
		t.Errorf("error = %v, want %v", err, errQuery)
	}
}

// -------------------------------------------------------------------------
// ROUTES
// -------------------------------------------------------------------------

// TestGetFlightRoute_Maps verifies the row maps onto route.Info and that the
// staleness cutoff comes off the store's configured route TTL. Note the
// callsign column populates FlightICAO; FlightIATA has no column and stays
// zero.
func TestGetFlightRoute_Maps(t *testing.T) {
	fake := &fakeQuerier{routeRow: db.GetFlightRouteRow{
		Callsign: "SWA123",
		DepIata:  "LAX",
		DepIcao:  "KLAX",
		DepName:  "Los Angeles Intl",
		ArrIata:  "PHX",
		ArrIcao:  "KPHX",
		ArrName:  "Phoenix Sky Harbor",
	}}

	got, err := newFakeStore(fake).GetFlightRoute(context.Background(), "SWA123")
	if err != nil {
		t.Fatalf("GetFlightRoute() error = %v", err)
	}

	want := &route.Info{
		FlightICAO: "SWA123",
		DepIATA:    "LAX",
		DepICAO:    "KLAX",
		DepName:    "Los Angeles Intl",
		ArrIATA:    "PHX",
		ArrICAO:    "KPHX",
		ArrName:    "Phoenix Sky Harbor",
	}
	if *got != *want {
		t.Errorf("GetFlightRoute() = %+v, want %+v", *got, *want)
	}

	if fake.gotRoute.Callsign != "SWA123" {
		t.Errorf("queried callsign = %q, want %q", fake.gotRoute.Callsign, "SWA123")
	}
	assertCutoff(t, fake.gotRoute.CachedAt, DefaultRouteTTL)
}

// TestGetFlightRoute_UsesConfiguredTTL pins the cutoff to the store's own
// routeTTL rather than the package default, which is the field NewPostgresStore
// lets a caller override.
func TestGetFlightRoute_UsesConfiguredTTL(t *testing.T) {
	fake := &fakeQuerier{routeErr: pgx.ErrNoRows}
	store := newFakeStore(fake)
	store.routeTTL = 90 * time.Minute

	if _, err := store.GetFlightRoute(context.Background(), "SWA123"); err != nil {
		t.Fatalf("GetFlightRoute() error = %v", err)
	}
	assertCutoff(t, fake.gotRoute.CachedAt, 90*time.Minute)
}

// TestGetFlightRoute_Errors mirrors the aircraft case: a miss is nil, anything
// else propagates.
func TestGetFlightRoute_Errors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantErr error
	}{
		{"no rows is a miss, not an error", pgx.ErrNoRows, nil},
		{"driver error propagates", errQuery, errQuery},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newFakeStore(&fakeQuerier{routeErr: tt.err}).
				GetFlightRoute(context.Background(), "SWA123")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if got != nil {
				t.Errorf("result = %+v, want nil", got)
			}
		})
	}
}

// TestSaveFlightRoute_Maps verifies FlightICAO is what lands in the callsign
// column. Routes are keyed by callsign, so picking the wrong source field here
// would write rows nothing ever reads back.
func TestSaveFlightRoute_Maps(t *testing.T) {
	fake := &fakeQuerier{}
	info := &route.Info{
		FlightICAO: "SWA123",
		FlightIATA: "WN123",
		DepIATA:    "LAX",
		DepICAO:    "KLAX",
		DepName:    "Los Angeles Intl",
		ArrIATA:    "PHX",
		ArrICAO:    "KPHX",
		ArrName:    "Phoenix Sky Harbor",
	}
	if err := newFakeStore(fake).SaveFlightRoute(context.Background(), info); err != nil {
		t.Fatalf("SaveFlightRoute() error = %v", err)
	}

	want := db.UpsertFlightRouteParams{
		Callsign: "SWA123",
		DepIata:  "LAX",
		DepIcao:  "KLAX",
		DepName:  "Los Angeles Intl",
		ArrIata:  "PHX",
		ArrIcao:  "KPHX",
		ArrName:  "Phoenix Sky Harbor",
	}
	if fake.gotRouteSave != want {
		t.Errorf("upsert params = %+v, want %+v", fake.gotRouteSave, want)
	}
}

// -------------------------------------------------------------------------
// SIGHTINGS
// -------------------------------------------------------------------------

// TestLogSighting_Maps verifies the position arguments land in the right
// columns and that the row is stamped with the store's clock, not the
// database's.
func TestLogSighting_Maps(t *testing.T) {
	fake := &fakeQuerier{}
	if err := newFakeStore(fake).LogSighting(context.Background(), "a1b2c3", 34.05, -118.25, 12.5); err != nil {
		t.Fatalf("LogSighting() error = %v", err)
	}

	got := fake.gotSighting
	if got.Icao24 != "a1b2c3" || got.Lat != 34.05 || got.Lon != -118.25 || got.DistanceKm != 12.5 {
		t.Errorf("sighting params = %+v, want icao24=a1b2c3 lat=34.05 lon=-118.25 dist=12.5", got)
	}
	assertCutoff(t, got.SeenAt, 0)
}

// -------------------------------------------------------------------------
// SQUAWK ALERTS
// -------------------------------------------------------------------------

// TestHasRecentSquawkAlert verifies the cooldown window is measured back from
// the store's clock and that the boolean and any error pass straight through.
func TestHasRecentSquawkAlert(t *testing.T) {
	tests := []struct {
		name     string
		has      bool
		queryErr error
	}{
		{"inside cooldown", true, nil},
		{"outside cooldown", false, nil},
		{"driver error propagates", false, errQuery},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeQuerier{hasAlert: tt.has, hasAlertErr: tt.queryErr}
			got, err := newFakeStore(fake).
				HasRecentSquawkAlert(context.Background(), "a1b2c3", "7700", 30*time.Minute)

			if !errors.Is(err, tt.queryErr) {
				t.Fatalf("error = %v, want %v", err, tt.queryErr)
			}
			if got != tt.has {
				t.Errorf("HasRecentSquawkAlert() = %v, want %v", got, tt.has)
			}
			if fake.gotHasAlert.Icao24 != "a1b2c3" || fake.gotHasAlert.Squawk != "7700" {
				t.Errorf("params = %+v, want icao24=a1b2c3 squawk=7700", fake.gotHasAlert)
			}
			assertCutoff(t, fake.gotHasAlert.SeenAt, 30*time.Minute)
		})
	}
}

// TestInsertSquawkAlert_Maps verifies the alert fields and the clock stamp.
func TestInsertSquawkAlert_Maps(t *testing.T) {
	fake := &fakeQuerier{}
	err := newFakeStore(fake).
		InsertSquawkAlert(context.Background(), "a1b2c3", "SWA123", "7700", 34.05, -118.25)
	if err != nil {
		t.Fatalf("InsertSquawkAlert() error = %v", err)
	}

	got := fake.gotInsert
	if got.Icao24 != "a1b2c3" || got.Callsign != "SWA123" || got.Squawk != "7700" {
		t.Errorf("alert params = %+v, want icao24=a1b2c3 callsign=SWA123 squawk=7700", got)
	}
	if got.Lat != 34.05 || got.Lon != -118.25 {
		t.Errorf("alert position = (%v, %v), want (34.05, -118.25)", got.Lat, got.Lon)
	}
	assertCutoff(t, got.SeenAt, 0)
}

// TestGetRecentSquawkAlerts_Maps verifies each generated row becomes a domain
// alert with the timestamp unwrapped out of pgtype, and that the lookback is
// measured from the store's clock.
func TestGetRecentSquawkAlerts_Maps(t *testing.T) {
	seen := testNow.Add(-10 * time.Minute)
	fake := &fakeQuerier{alertRows: []db.SquawkAlert{
		{ID: 1, Icao24: "a1b2c3", Callsign: "SWA123", Squawk: "7700", Lat: 34.05, Lon: -118.25,
			SeenAt: pgtype.Timestamptz{Time: seen, Valid: true}},
		{ID: 2, Icao24: "d4e5f6", Callsign: "AAL456", Squawk: "7600", Lat: 33.94, Lon: -118.40,
			SeenAt: pgtype.Timestamptz{Time: seen, Valid: true}},
	}}

	got, err := newFakeStore(fake).GetRecentSquawkAlerts(context.Background(), time.Hour)
	if err != nil {
		t.Fatalf("GetRecentSquawkAlerts() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d alerts, want 2", len(got))
	}
	if got[0].ID != 1 || got[0].ICAO24 != "a1b2c3" || got[0].Callsign != "SWA123" || got[0].Squawk != "7700" {
		t.Errorf("alert[0] = %+v, want id=1 icao24=a1b2c3 callsign=SWA123 squawk=7700", got[0])
	}
	if !got[0].SeenAt.Equal(seen) {
		t.Errorf("alert[0].SeenAt = %v, want %v", got[0].SeenAt, seen)
	}
	if got[1].ID != 2 || got[1].ICAO24 != "d4e5f6" {
		t.Errorf("alert[1] = %+v, want id=2 icao24=d4e5f6", got[1])
	}
	assertCutoff(t, fake.gotSince, time.Hour)
}

// TestGetRecentSquawkAlerts_Empty verifies no rows yields an empty, non-nil
// slice: the dashboard marshals this straight to JSON, where nil would render
// as null rather than [].
func TestGetRecentSquawkAlerts_NoRows(t *testing.T) {
	got, err := newFakeStore(&fakeQuerier{}).GetRecentSquawkAlerts(context.Background(), time.Hour)
	if err != nil {
		t.Fatalf("GetRecentSquawkAlerts() error = %v", err)
	}
	if got == nil {
		t.Fatal("result is nil, want empty slice")
	}
	if len(got) != 0 {
		t.Errorf("got %d alerts, want 0", len(got))
	}
}

// TestGetRecentSquawkAlerts_Error verifies a driver failure is surfaced rather
// than reported as an empty result.
func TestGetRecentSquawkAlerts_Error(t *testing.T) {
	got, err := newFakeStore(&fakeQuerier{alertErr: errQuery}).
		GetRecentSquawkAlerts(context.Background(), time.Hour)
	if !errors.Is(err, errQuery) {
		t.Fatalf("error = %v, want %v", err, errQuery)
	}
	if got != nil {
		t.Errorf("result = %+v, want nil", got)
	}
}

// -------------------------------------------------------------------------
// RETENTION
// -------------------------------------------------------------------------

// TestDeleteOlderThan covers the three retention methods together: each is the
// same wrapper over a different generated query, so the thing worth pinning is
// that each one measures its cutoff back from the clock and reports the row
// count out of the command tag.
func TestDeleteOlderThan(t *testing.T) {
	tests := []struct {
		name string
		call func(*PostgresStore, context.Context, time.Duration) (int64, error)
	}{
		{"sightings", (*PostgresStore).DeleteOldSightings},
		{"squawk alerts", (*PostgresStore).DeleteOldSquawkAlerts},
		{"routes", (*PostgresStore).DeleteOldRoutes},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeQuerier{tag: pgconn.NewCommandTag("DELETE 7")}
			got, err := tt.call(newFakeStore(fake), context.Background(), 48*time.Hour)
			if err != nil {
				t.Fatalf("delete error = %v", err)
			}
			if got != 7 {
				t.Errorf("rows deleted = %d, want 7", got)
			}
			assertCutoff(t, fake.gotCutoff, 48*time.Hour)
		})
	}
}

// TestDeleteOlderThan_Error verifies a failed delete reports zero rows
// alongside the error, so a caller that logs the count cannot mistake a
// failure for a no-op sweep.
func TestDeleteOlderThan_Error(t *testing.T) {
	fake := &fakeQuerier{tagErr: errQuery}
	got, err := newFakeStore(fake).DeleteOldSightings(context.Background(), time.Hour)
	if !errors.Is(err, errQuery) {
		t.Fatalf("error = %v, want %v", err, errQuery)
	}
	if got != 0 {
		t.Errorf("rows deleted = %d, want 0", got)
	}
}
