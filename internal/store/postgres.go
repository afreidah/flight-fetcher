// -------------------------------------------------------------------------------
// Store - PostgreSQL Aircraft Metadata and History
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Manages persistent aircraft metadata cache and historical sighting log in
// PostgreSQL. Aircraft metadata is cached indefinitely on first enrichment.
// Sightings are logged each poll cycle for historical analysis.
// -------------------------------------------------------------------------------

package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/afreidah/flight-fetcher/internal/hexdb"

	_ "github.com/lib/pq"
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// PostgresStore manages aircraft metadata and sighting history in PostgreSQL.
type PostgresStore struct {
	db *sql.DB
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// NewPostgresStore opens a connection to PostgreSQL using the given DSN and
// verifies connectivity.
func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}
	return &PostgresStore{db: db}, nil
}

// SaveAircraftMeta caches aircraft metadata, upserting by ICAO24.
func (p *PostgresStore) SaveAircraftMeta(ctx context.Context, info *hexdb.AircraftInfo) error {
	query := `
		INSERT INTO aircraft_meta (icao24, registration, manufacturer, type, operator)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (icao24) DO UPDATE SET
			registration = EXCLUDED.registration,
			manufacturer = EXCLUDED.manufacturer,
			type = EXCLUDED.type,
			operator = EXCLUDED.operator`
	_, err := p.db.ExecContext(ctx, query,
		info.ICAO24, info.Registration, info.ManufacturerName, info.Type, info.OperatorFlagCode)
	return err
}

// GetAircraftMeta retrieves cached aircraft metadata by ICAO24. Returns nil
// if the aircraft has not been enriched yet.
func (p *PostgresStore) GetAircraftMeta(ctx context.Context, icao24 string) (*hexdb.AircraftInfo, error) {
	var info hexdb.AircraftInfo
	err := p.db.QueryRowContext(ctx,
		`SELECT icao24, registration, manufacturer, type, operator FROM aircraft_meta WHERE icao24 = $1`,
		icao24).Scan(&info.ICAO24, &info.Registration, &info.ManufacturerName, &info.Type, &info.OperatorFlagCode)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// LogSighting records a historical sighting of an aircraft at a given
// position and distance from the configured center point.
func (p *PostgresStore) LogSighting(ctx context.Context, icao24 string, lat, lon, distanceKm float64) error {
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO sightings (icao24, lat, lon, distance_km, seen_at) VALUES ($1, $2, $3, $4, $5)`,
		icao24, lat, lon, distanceKm, time.Now().UTC())
	return err
}

// Close shuts down the PostgreSQL connection pool.
func (p *PostgresStore) Close() error {
	return p.db.Close()
}
