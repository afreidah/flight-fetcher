// -------------------------------------------------------------------------------
// Store - PostgreSQL Aircraft Metadata and History
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Manages persistent aircraft metadata cache and historical sighting log in
// PostgreSQL via pgx connection pool and sqlc-generated queries. Runs goose
// migrations on startup to ensure the schema is current.
// -------------------------------------------------------------------------------

package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/afreidah/flight-fetcher/internal/airlabs"
	"github.com/afreidah/flight-fetcher/internal/hexdb"
	"github.com/afreidah/flight-fetcher/internal/store/migrations"
	db "github.com/afreidah/flight-fetcher/internal/store/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// PostgresStore manages aircraft metadata and sighting history in PostgreSQL.
type PostgresStore struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// NewPostgresStore opens a connection pool to PostgreSQL, runs pending
// migrations, and returns a ready-to-use store.
func NewPostgresStore(ctx context.Context, dsn string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	if err := runMigrations(dsn); err != nil {
		pool.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return &PostgresStore{
		pool:    pool,
		queries: db.New(pool),
	}, nil
}

// SaveAircraftMeta caches aircraft metadata, upserting by ICAO24.
func (p *PostgresStore) SaveAircraftMeta(ctx context.Context, info *hexdb.AircraftInfo) error {
	return p.queries.UpsertAircraftMeta(ctx, db.UpsertAircraftMetaParams{
		Icao24:       info.ICAO24,
		Registration: info.Registration,
		Manufacturer: info.ManufacturerName,
		Type:         info.Type,
		Operator:     info.OperatorFlagCode,
	})
}

// GetAircraftMeta retrieves cached aircraft metadata by ICAO24. Returns nil
// if the aircraft has not been enriched yet.
func (p *PostgresStore) GetAircraftMeta(ctx context.Context, icao24 string) (*hexdb.AircraftInfo, error) {
	row, err := p.queries.GetAircraftMeta(ctx, icao24)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &hexdb.AircraftInfo{
		ICAO24:           row.Icao24,
		Registration:     row.Registration,
		ManufacturerName: row.Manufacturer,
		Type:             row.Type,
		OperatorFlagCode: row.Operator,
	}, nil
}

// LogSighting records a historical sighting of an aircraft at a given
// position and distance from the configured center point.
func (p *PostgresStore) LogSighting(ctx context.Context, icao24 string, lat, lon, distanceKm float64) error {
	return p.queries.LogSighting(ctx, db.LogSightingParams{
		Icao24:     icao24,
		Lat:        lat,
		Lon:        lon,
		DistanceKm: distanceKm,
		SeenAt: pgtype.Timestamptz{
			Time:  time.Now().UTC(),
			Valid: true,
		},
	})
}

// SaveFlightRoute caches flight route information, upserting by callsign.
func (p *PostgresStore) SaveFlightRoute(ctx context.Context, route *airlabs.FlightRoute) error {
	return p.queries.UpsertFlightRoute(ctx, db.UpsertFlightRouteParams{
		Callsign: route.FlightICAO,
		DepIata:  route.DepIATA,
		DepIcao:  route.DepICAO,
		DepName:  route.DepName,
		ArrIata:  route.ArrIATA,
		ArrIcao:  route.ArrICAO,
		ArrName:  route.ArrName,
	})
}

// GetFlightRoute retrieves cached route information by callsign. Returns nil
// if the route has not been looked up yet.
func (p *PostgresStore) GetFlightRoute(ctx context.Context, callsign string) (*airlabs.FlightRoute, error) {
	row, err := p.queries.GetFlightRoute(ctx, callsign)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &airlabs.FlightRoute{
		FlightICAO: row.Callsign,
		DepIATA:    row.DepIata,
		DepICAO:    row.DepIcao,
		DepName:    row.DepName,
		ArrIATA:    row.ArrIata,
		ArrICAO:    row.ArrIcao,
		ArrName:    row.ArrName,
	}, nil
}

// InsertSquawkAlert records an emergency squawk detection.
func (p *PostgresStore) InsertSquawkAlert(ctx context.Context, icao24, callsign, squawk string, lat, lon float64) error {
	return p.queries.InsertSquawkAlert(ctx, db.InsertSquawkAlertParams{
		Icao24:   icao24,
		Callsign: callsign,
		Squawk:   squawk,
		Lat:      lat,
		Lon:      lon,
		SeenAt: pgtype.Timestamptz{
			Time:  time.Now().UTC(),
			Valid: true,
		},
	})
}

// GetRecentSquawkAlerts returns squawk alerts from the last given duration.
func (p *PostgresStore) GetRecentSquawkAlerts(ctx context.Context, since time.Duration) ([]db.SquawkAlert, error) {
	return p.queries.GetRecentSquawkAlerts(ctx, pgtype.Timestamptz{
		Time:  time.Now().UTC().Add(-since),
		Valid: true,
	})
}

// Close shuts down the PostgreSQL connection pool.
func (p *PostgresStore) Close() {
	p.pool.Close()
}

// -------------------------------------------------------------------------
// INTERNALS
// -------------------------------------------------------------------------

// runMigrations opens a standard database/sql connection and applies any
// pending goose migrations from the embedded migration files.
func runMigrations(dsn string) error {
	stdDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("opening migration connection: %w", err)
	}
	defer stdDB.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, stdDB, migrations.FS)
	if err != nil {
		return fmt.Errorf("creating migration provider: %w", err)
	}

	ctx := context.Background()
	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("applying migrations: %w", err)
	}

	for _, r := range results {
		slog.InfoContext(ctx, "migration applied",
			slog.String("file", r.Source.Path),
			slog.String("duration", r.Duration.String()))
	}
	return nil
}
