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
	"github.com/afreidah/flight-fetcher/internal/geo"
	"github.com/afreidah/flight-fetcher/internal/hexdb"
	"github.com/afreidah/flight-fetcher/internal/opensky"
	"github.com/afreidah/flight-fetcher/internal/poller"
	"github.com/afreidah/flight-fetcher/internal/server"
	"github.com/afreidah/flight-fetcher/internal/store"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	configPath := flag.String("config", "config.hcl", "path to config file")
	flag.Parse()

	ctx := context.Background()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.ErrorContext(ctx, "failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	pollInterval, err := cfg.PollDuration()
	if err != nil {
		slog.ErrorContext(ctx, "failed to parse poll interval", slog.String("error", err.Error()))
		os.Exit(1)
	}

	oskyClient := opensky.NewClient(cfg.OpenSky.ID, cfg.OpenSky.Secret)
	hexdbClient := hexdb.NewClient()

	redisStore := store.NewRedisStore(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	defer redisStore.Close()

	pgStore, err := store.NewPostgresStore(ctx, cfg.Postgres.DSN)
	if err != nil {
		slog.ErrorContext(ctx, "failed to connect to postgres", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pgStore.Close()

	var routeLookup enricher.RouteLookup
	var routeStore enricher.RouteStore
	if cfg.AirLabs != nil && cfg.AirLabs.APIKey != "" {
		routeLookup = airlabs.NewClient(cfg.AirLabs.APIKey)
		routeStore = pgStore
		slog.InfoContext(ctx, "airlabs route enrichment enabled")
	}

	enr := enricher.New(hexdbClient, pgStore, routeLookup, routeStore)
	center := geo.Coord{Lat: cfg.Location.Lat, Lon: cfg.Location.Lon}
	p := poller.New(oskyClient, redisStore, pgStore, enr, center, cfg.Location.RadiusKm, pollInterval)


	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if cfg.Server != nil && cfg.Server.Listen != "" {
		srv := server.New(redisStore, pgStore, pgStore)
		go srv.ListenAndServe(ctx, cfg.Server.Listen)
	}

	p.Run(ctx)
}
