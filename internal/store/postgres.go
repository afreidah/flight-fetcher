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
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/afreidah/flight-fetcher/internal/aircraft"
	"github.com/afreidah/flight-fetcher/internal/route"
	"github.com/afreidah/flight-fetcher/internal/squawk"
	"github.com/afreidah/flight-fetcher/internal/store/migrations"
	db "github.com/afreidah/flight-fetcher/internal/store/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

const (
	// DefaultRouteTTL is the default time before cached routes are considered stale.
	DefaultRouteTTL = 24 * time.Hour
	// DefaultAircraftTTL is the default time before cached aircraft metadata
	// is considered stale and re-enriched.
	DefaultAircraftTTL = 7 * 24 * time.Hour
)

// PostgresStore manages aircraft metadata and sighting history in PostgreSQL.
type PostgresStore struct {
	pool        *pgxpool.Pool
	queries     *db.Queries
	routeTTL    time.Duration
	aircraftTTL time.Duration
	tracer      trace.Tracer
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// NewPostgresStore opens a connection pool to PostgreSQL, runs pending
// migrations, and returns a ready-to-use store. The routeTTL controls how
// long cached routes are considered fresh (0 uses DefaultRouteTTL).
func NewPostgresStore(ctx context.Context, dsn string, routeTTL time.Duration) (*PostgresStore, error) {
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing dsn: %w", err)
	}
	poolCfg.MaxConns = 10
	poolCfg.MinConns = 2
	poolCfg.MaxConnLifetime = 30 * time.Minute
	poolCfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	if err := runMigrations(ctx, dsn); err != nil {
		pool.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	if routeTTL <= 0 {
		routeTTL = DefaultRouteTTL
	}
	return &PostgresStore{
		pool:        pool,
		queries:     db.New(pool),
		routeTTL:    routeTTL,
		aircraftTTL: DefaultAircraftTTL,
		tracer:      otel.Tracer("flight-fetcher/postgres"),
	}, nil
}

// startSpan creates a child span and returns the updated context and end
// function. The end function records any non-nil error on the span.
func (p *PostgresStore) startSpan(ctx context.Context, name string) (context.Context, func(error)) {
	ctx, span := p.tracer.Start(ctx, "postgres."+name)
	return ctx, func(err error) {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}
}

// traced wraps a database operation in a tracing span, recording any
// non-nil error. Used for read and delete methods that return (T, error).
func traced[T any](p *PostgresStore, ctx context.Context, name string, fn func(context.Context) (T, error)) (T, error) {
	ctx, endSpan := p.startSpan(ctx, name)
	result, err := fn(ctx)
	endSpan(err)
	return result, err
}

// deleteOlderThan wraps a batched delete query in a tracing span,
// converting the CommandTag result to a row count.
func deleteOlderThan(p *PostgresStore, ctx context.Context, name string, maxAge time.Duration, fn func(context.Context, pgtype.Timestamptz) (pgconn.CommandTag, error)) (int64, error) {
	return traced(p, ctx, name, func(ctx context.Context) (int64, error) {
		result, err := fn(ctx, pgtype.Timestamptz{Time: time.Now().UTC().Add(-maxAge), Valid: true})
		if err != nil {
			return 0, err
		}
		return result.RowsAffected(), nil
	})
}

// SaveAircraftMeta caches aircraft metadata, upserting by ICAO24.
func (p *PostgresStore) SaveAircraftMeta(ctx context.Context, info *aircraft.Info) error {
	ctx, endSpan := p.startSpan(ctx, "SaveAircraftMeta")
	err := p.queries.UpsertAircraftMeta(ctx, db.UpsertAircraftMetaParams{
		Icao24:           info.ICAO24,
		Registration:     info.Registration,
		Manufacturer:     info.ManufacturerName,
		Type:             info.Type,
		Operator:         info.OperatorFlagCode,
		IcaoTypeCode:     info.ICAOTypeCode,
		RegisteredOwners: info.RegisteredOwners,
		ImageUrl:         info.ImageURL,
	})
	endSpan(err)
	return err
}

// GetAircraftMeta retrieves cached aircraft metadata by ICAO24. Returns nil
// if the aircraft has not been enriched yet or the cached entry is stale.
func (p *PostgresStore) GetAircraftMeta(ctx context.Context, icao24 string) (*aircraft.Info, error) {
	return traced(p, ctx, "GetAircraftMeta", func(ctx context.Context) (*aircraft.Info, error) {
		row, err := p.queries.GetAircraftMeta(ctx, db.GetAircraftMetaParams{
			Icao24:    icao24,
			UpdatedAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(-p.aircraftTTL), Valid: true},
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return &aircraft.Info{
			ICAO24:           row.Icao24,
			Registration:     row.Registration,
			ManufacturerName: row.Manufacturer,
			Type:             row.Type,
			OperatorFlagCode: row.Operator,
			ICAOTypeCode:     row.IcaoTypeCode,
			RegisteredOwners: row.RegisteredOwners,
			ImageURL:         row.ImageUrl,
		}, nil
	})
}

// LogSighting records a historical sighting of an aircraft at a given
// position and distance from the configured center point.
func (p *PostgresStore) LogSighting(ctx context.Context, icao24 string, lat, lon, distanceKm float64) error {
	ctx, endSpan := p.startSpan(ctx, "LogSighting")
	err := p.queries.LogSighting(ctx, db.LogSightingParams{
		Icao24:     icao24,
		Lat:        lat,
		Lon:        lon,
		DistanceKm: distanceKm,
		SeenAt: pgtype.Timestamptz{
			Time:  time.Now().UTC(),
			Valid: true,
		},
	})
	endSpan(err)
	return err
}

// SaveFlightRoute caches flight route information, upserting by callsign.
func (p *PostgresStore) SaveFlightRoute(ctx context.Context, route *route.Info) error {
	ctx, endSpan := p.startSpan(ctx, "SaveFlightRoute")
	err := p.queries.UpsertFlightRoute(ctx, db.UpsertFlightRouteParams{
		Callsign: route.FlightICAO,
		DepIata:  route.DepIATA,
		DepIcao:  route.DepICAO,
		DepName:  route.DepName,
		ArrIata:  route.ArrIATA,
		ArrIcao:  route.ArrICAO,
		ArrName:  route.ArrName,
	})
	endSpan(err)
	return err
}

// GetFlightRoute retrieves cached route information by callsign. Returns nil
// if the route has not been looked up yet or the cached entry is stale.
func (p *PostgresStore) GetFlightRoute(ctx context.Context, callsign string) (*route.Info, error) {
	return traced(p, ctx, "GetFlightRoute", func(ctx context.Context) (*route.Info, error) {
		row, err := p.queries.GetFlightRoute(ctx, db.GetFlightRouteParams{
			Callsign: callsign,
			CachedAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(-p.routeTTL), Valid: true},
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return &route.Info{
			FlightICAO: row.Callsign,
			DepIATA:    row.DepIata,
			DepICAO:    row.DepIcao,
			DepName:    row.DepName,
			ArrIATA:    row.ArrIata,
			ArrICAO:    row.ArrIcao,
			ArrName:    row.ArrName,
		}, nil
	})
}

// HasRecentSquawkAlert checks if an alert for the given icao24 and squawk
// code exists within the cooldown window.
func (p *PostgresStore) HasRecentSquawkAlert(ctx context.Context, icao24, squawk string, cooldown time.Duration) (bool, error) {
	return traced(p, ctx, "HasRecentSquawkAlert", func(ctx context.Context) (bool, error) {
		return p.queries.HasRecentSquawkAlert(ctx, db.HasRecentSquawkAlertParams{
			Icao24: icao24,
			Squawk: squawk,
			SeenAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(-cooldown), Valid: true},
		})
	})
}

// InsertSquawkAlert records an emergency squawk detection.
func (p *PostgresStore) InsertSquawkAlert(ctx context.Context, icao24, callsign, squawk string, lat, lon float64) error {
	ctx, endSpan := p.startSpan(ctx, "InsertSquawkAlert")
	err := p.queries.InsertSquawkAlert(ctx, db.InsertSquawkAlertParams{
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
	endSpan(err)
	return err
}

// GetRecentSquawkAlerts returns squawk alerts from the last given duration.
func (p *PostgresStore) GetRecentSquawkAlerts(ctx context.Context, since time.Duration) ([]squawk.Alert, error) {
	return traced(p, ctx, "GetRecentSquawkAlerts", func(ctx context.Context) ([]squawk.Alert, error) {
		rows, err := p.queries.GetRecentSquawkAlerts(ctx, pgtype.Timestamptz{
			Time:  time.Now().UTC().Add(-since),
			Valid: true,
		})
		if err != nil {
			return nil, err
		}
		alerts := make([]squawk.Alert, len(rows))
		for i, r := range rows {
			alerts[i] = squawk.Alert{
				ID:       r.ID,
				ICAO24:   r.Icao24,
				Callsign: r.Callsign,
				Squawk:   r.Squawk,
				Lat:      r.Lat,
				Lon:      r.Lon,
				SeenAt:   r.SeenAt.Time,
			}
		}
		return alerts, nil
	})
}

// DeleteOldSightings removes sightings older than the given duration.
// Returns the number of rows deleted.
func (p *PostgresStore) DeleteOldSightings(ctx context.Context, maxAge time.Duration) (int64, error) {
	return deleteOlderThan(p, ctx, "DeleteOldSightings", maxAge, p.queries.DeleteOldSightings)
}

// DeleteOldSquawkAlerts removes squawk alerts older than the given duration.
// Returns the number of rows deleted.
func (p *PostgresStore) DeleteOldSquawkAlerts(ctx context.Context, maxAge time.Duration) (int64, error) {
	return deleteOlderThan(p, ctx, "DeleteOldSquawkAlerts", maxAge, p.queries.DeleteOldSquawkAlerts)
}

// DeleteOldRoutes removes cached routes older than the given duration.
// Returns the number of rows deleted.
func (p *PostgresStore) DeleteOldRoutes(ctx context.Context, maxAge time.Duration) (int64, error) {
	return deleteOlderThan(p, ctx, "DeleteOldRoutes", maxAge, p.queries.DeleteOldRoutes)
}

// Ping verifies the PostgreSQL connection is alive.
func (p *PostgresStore) Ping(ctx context.Context) error {
	ctx, endSpan := p.startSpan(ctx, "Ping")
	err := p.pool.Ping(ctx)
	endSpan(err)
	return err
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
func runMigrations(ctx context.Context, dsn string) error {
	stdDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("opening migration connection: %w", err)
	}
	defer stdDB.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, stdDB, migrations.FS)
	if err != nil {
		return fmt.Errorf("creating migration provider: %w", err)
	}

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
