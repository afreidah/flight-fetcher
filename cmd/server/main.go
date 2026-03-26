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
	"time"

	"github.com/afreidah/flight-fetcher/internal/aircraft"
	"github.com/afreidah/flight-fetcher/internal/apiclient/airlabs"
	"github.com/afreidah/flight-fetcher/internal/config"
	"github.com/afreidah/flight-fetcher/internal/enricher"
	"github.com/afreidah/flight-fetcher/internal/notify"
	"github.com/afreidah/flight-fetcher/internal/notify/discord"
	"github.com/afreidah/flight-fetcher/internal/notify/telegram"
	"github.com/afreidah/flight-fetcher/internal/observe"
	"github.com/afreidah/flight-fetcher/internal/apiclient/flightaware"
	"github.com/afreidah/flight-fetcher/internal/geo"
	"github.com/afreidah/flight-fetcher/internal/apiclient/hexdb"
	"github.com/afreidah/flight-fetcher/internal/apiclient/opensky"
	"github.com/afreidah/flight-fetcher/internal/poller"
	"github.com/afreidah/flight-fetcher/internal/retention"
	"github.com/afreidah/flight-fetcher/internal/route"
	"github.com/afreidah/flight-fetcher/internal/server"
	"github.com/afreidah/flight-fetcher/internal/squawk"
	"github.com/afreidah/flight-fetcher/internal/apiclient/dump1090"
	"github.com/afreidah/flight-fetcher/internal/store"
	"github.com/afreidah/flight-fetcher/internal/tfr"

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
	ctx := context.Background()

	otelShutdown, err := observe.Setup(ctx, "flight-fetcher", Version)
	if err != nil {
		slog.ErrorContext(ctx, "failed to initialize observability", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		if err := otelShutdown(ctx); err != nil {
			slog.ErrorContext(ctx, "observability shutdown error", slog.String("error", err.Error()))
		}
	}()

	slog.SetDefault(slog.New(&observe.TracedHandler{
		Handler: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}),
	}))

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

	oskyLookupClient := opensky.NewClient(cfg.OpenSky.ID, cfg.OpenSky.Secret)
	aircraftSources := []enricher.NamedSource[aircraft.Info]{
		{Name: "hexdb", Fn: hexdbClient.Lookup},
		{Name: "opensky", Fn: oskyLookupClient.Lookup},
	}

	var routeSources []enricher.NamedSource[route.Info]
	if cfg.AirLabs != nil && cfg.AirLabs.APIKey != "" {
		routeSources = append(routeSources, enricher.NamedSource[route.Info]{
			Name: "airlabs", Fn: airlabs.NewClient(cfg.AirLabs.APIKey).LookupRoute,
		})
		slog.InfoContext(ctx, "airlabs route enrichment enabled")
	}
	if cfg.FlightAware != nil && cfg.FlightAware.APIKey != "" {
		routeSources = append(routeSources, enricher.NamedSource[route.Info]{
			Name: "flightaware", Fn: flightaware.NewClient(cfg.FlightAware.APIKey).LookupRoute,
		})
		slog.InfoContext(ctx, "flightaware route enrichment enabled")
	}

	enr := enricher.New(&enricher.Options{
		AircraftSources: aircraftSources,
		Store:           pgStore,
		RouteSources:    routeSources,
		RouteStore:      pgStore,
	})
	var flightSource poller.FlightSource = oskyClient
	if cfg.Dump1090 != nil {
		flightSource = dump1090.NewClient(cfg.Dump1090.URL)
		slog.InfoContext(ctx, "using dump1090 as flight source", slog.String("url", cfg.Dump1090.URL))
	}

	p := poller.New(&poller.Options{
		Source:        flightSource,
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

	tfrCache := tfr.NewCache()
	g.Go(func() error { tfrCache.Run(ctx, 15*time.Minute); return nil })

	if cfg.Server != nil && cfg.Server.Listen != "" {
		srv := server.New(&server.Options{
			Flights:    redisStore,
			Aircraft:   pgStore,
			Routes:     pgStore,
			Alerts:     pgStore,
			TFRs:       tfrCache,
			Images:     hexdbClient,
			Pingers: []server.HealthPinger{
				{Name: "redis", Pinger: redisStore},
				{Name: "postgres", Pinger: pgStore},
			},
			Version:    Version,
			RefreshSec: cfg.Server.RefreshSeconds(),
		})
		g.Go(func() error { return srv.ListenAndServe(ctx, cfg.Server.Listen) })
	}

	if cfg.SquawkMonitor != nil {
		notifyMgr := notify.NewManager()
		if cfg.Notifications != nil {
			for _, d := range cfg.Notifications.Discord {
				notifyMgr.Register(discord.New(d.WebhookURL))
				slog.InfoContext(ctx, "discord notifications enabled")
			}
			for _, t := range cfg.Notifications.Telegram {
				notifyMgr.Register(telegram.New(t.BotToken, t.ChatID))
				slog.InfoContext(ctx, "telegram notifications enabled")
			}
		}
		squawkClient := opensky.NewClient(cfg.OpenSky.ID, cfg.OpenSky.Secret)
		sm := squawk.New(squawkClient, pgStore, enr, notifyMgr, cfg.SquawkMonitor.Poll)
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
