// -------------------------------------------------------------------------------
// Flight Fetcher - Server Entrypoint
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Loads configuration, initializes API clients and data stores, wires the
// polling pipeline, and runs until interrupted. OpenSky credentials are read
// from the config file, which is rendered by Vault in production.
// -------------------------------------------------------------------------------

package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/afreidah/flight-fetcher/internal/airlabs"
	"github.com/afreidah/flight-fetcher/internal/config"
	"github.com/afreidah/flight-fetcher/internal/enricher"
	"github.com/afreidah/flight-fetcher/internal/flightaware"
	"github.com/afreidah/flight-fetcher/internal/geo"
	"github.com/afreidah/flight-fetcher/internal/hexdb"
	"github.com/afreidah/flight-fetcher/internal/opensky"
	"github.com/afreidah/flight-fetcher/internal/poller"
	"github.com/afreidah/flight-fetcher/internal/retention"
	"github.com/afreidah/flight-fetcher/internal/server"
	"github.com/afreidah/flight-fetcher/internal/squawk"
	"github.com/afreidah/flight-fetcher/internal/store"

	"golang.org/x/sync/errgroup"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	configPath := flag.String("config", "config.hcl", "path to config file")
	logLevel := flag.String("log-level", "info", "log level (debug, info, warn, error)")
	flag.Parse()

	var level slog.Level
	if err := level.UnmarshalText([]byte(*logLevel)); err != nil {
		slog.ErrorContext(context.Background(), "invalid log level", slog.String("level", *logLevel), slog.String("error", err.Error()))
		os.Exit(1)
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})))

	ctx := context.Background()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.ErrorContext(ctx, "failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	oskyClient := opensky.NewClient(cfg.OpenSky.ID, cfg.OpenSky.Secret)
	hexdbClient := hexdb.NewClient()

	redisTTL := cfg.Poll * 3
	redisStore := store.NewRedisStore(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB, redisTTL)
	defer redisStore.Close()

	if err := redisStore.Ping(ctx); err != nil {
		slog.ErrorContext(ctx, "failed to connect to redis", slog.String("error", err.Error()))
		os.Exit(1)
	}

	pgStore, err := store.NewPostgresStore(ctx, cfg.Postgres.DSN, 0)
	if err != nil {
		slog.ErrorContext(ctx, "failed to connect to postgres", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pgStore.Close()

	var routeLookup enricher.RouteLookup
	var routeFallback enricher.RouteLookup
	var routeStore enricher.RouteStore
	if cfg.AirLabs != nil && cfg.AirLabs.APIKey != "" {
		routeLookup = airlabs.NewClient(cfg.AirLabs.APIKey)
		routeStore = pgStore
		slog.InfoContext(ctx, "airlabs route enrichment enabled")
	}
	if cfg.FlightAware != nil && cfg.FlightAware.APIKey != "" {
		fa := flightaware.NewClient(cfg.FlightAware.APIKey)
		if routeLookup != nil {
			routeFallback = fa
			slog.InfoContext(ctx, "flightaware route fallback enabled")
		} else {
			routeLookup = fa
			routeStore = pgStore
			slog.InfoContext(ctx, "flightaware route enrichment enabled (primary)")
		}
	}

	enr := enricher.New(&enricher.Options{
		Lookup:        hexdbClient,
		Store:         pgStore,
		RouteLookup:   routeLookup,
		RouteFallback: routeFallback,
		RouteStore:    routeStore,
	})
	p := poller.New(&poller.Options{
		Source:        oskyClient,
		Cache:         redisStore,
		Logger:        pgStore,
		Enricher:      enr,
		Center:        geo.Coord{Lat: cfg.Location.Lat, Lon: cfg.Location.Lon},
		RadiusKm:      cfg.Location.RadiusKm,
		Interval:      cfg.Poll,
		EvictInterval: cfg.EnrichInterval,
	})

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	if cfg.Server != nil && cfg.Server.Listen != "" {
		srv := server.New(&server.Options{
			Flights:    redisStore,
			Aircraft:   pgStore,
			Routes:     pgStore,
			Alerts:     pgStore,
			Pingers:    []server.HealthPinger{redisStore, pgStore},
			Version:    Version,
			RefreshSec: cfg.Server.RefreshSeconds(),
		})
		g.Go(func() error { srv.ListenAndServe(ctx, cfg.Server.Listen); return nil })
	}

	if cfg.SquawkMonitor != nil {
		squawkClient := opensky.NewClient(cfg.OpenSky.ID, cfg.OpenSky.Secret)
		sm := squawk.New(squawkClient, pgStore, enr, cfg.SquawkMonitor.Poll)
		g.Go(func() error { sm.Run(ctx); return nil })
	}

	if cfg.Retention != nil {
		r := cfg.Retention
		rw := retention.New(pgStore, r.Sightings, r.Alerts, r.Routes, r.CleanInterval)
		g.Go(func() error { rw.Run(ctx); return nil })
	}

	g.Go(func() error { p.Run(ctx); return nil })

	if err := g.Wait(); err != nil {
		slog.ErrorContext(ctx, "shutdown error", slog.String("error", err.Error()))
	}
	slog.InfoContext(ctx, "shutdown complete")
}
