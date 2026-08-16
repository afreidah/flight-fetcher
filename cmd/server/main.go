// -------------------------------------------------------------------------------
// Flight Fetcher - Server Entrypoint
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Loads configuration, initializes API clients and data stores, wires the
// polling pipeline, and runs until interrupted. OpenSky credentials are read
// from the config file, which is rendered by Vault in production. The
// decisions about what to construct live in wiring.go; this file is the
// construction itself, kept linear so the startup order reads top to bottom.
// -------------------------------------------------------------------------------

package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/afreidah/flight-fetcher/internal/apiclient/hexdb"
	"github.com/afreidah/flight-fetcher/internal/apiclient/opensky"
	"github.com/afreidah/flight-fetcher/internal/config"
	"github.com/afreidah/flight-fetcher/internal/enricher"
	"github.com/afreidah/flight-fetcher/internal/geo"
	"github.com/afreidah/flight-fetcher/internal/notify"
	"github.com/afreidah/flight-fetcher/internal/observe"
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

	ctx := context.Background()
	cfg, otelShutdown := setup(ctx, *configPath, *logLevel)
	defer otelShutdown()

	specs := plannedSources(cfg)
	slowest, redisTTL := cacheTTL(specs)
	slog.InfoContext(ctx, "redis ttl computed from slowest poller",
		slog.Duration("slowest_interval", slowest),
		slog.Duration("redis_ttl", redisTTL))

	redisStore := store.NewRedisStore(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB, redisTTL)
	defer redisStore.Close()
	if err := redisStore.Ping(ctx); err != nil {
		fatal(ctx, "failed to connect to redis", err)
	}

	pgStore, err := store.NewPostgresStore(ctx, cfg.Postgres.DSN, 0)
	if err != nil {
		fatal(ctx, "failed to connect to postgres", err)
	}
	defer pgStore.Close()

	hexdbClient := hexdb.NewClient()
	routeSources := plannedRouteSources(cfg)
	for _, s := range routeSources {
		slog.InfoContext(ctx, "route enrichment enabled", slog.String("source", s.Name))
	}

	enr := enricher.New(&enricher.Options{
		AircraftSources: plannedAircraftSources(cfg, hexdbClient),
		Store:           pgStore,
		RouteSources:    routeSources,
		RouteStore:      pgStore,
	})

	dedup := poller.NewDedupState(cfg.EnrichInterval)
	center := geo.Coord{Lat: cfg.Location.Lat, Lon: cfg.Location.Lon}
	pollers := make([]*poller.Poller, 0, len(specs))
	for _, s := range specs {
		slog.InfoContext(ctx, "flight source enabled",
			slog.String("source", s.name),
			slog.Duration("interval", s.interval))
		pollers = append(pollers, poller.New(&poller.Options{
			Name:     s.name,
			Source:   s.source,
			Cache:    redisStore,
			Logger:   pgStore,
			Enricher: enr,
			Dedup:    dedup,
			Center:   center,
			RadiusKm: cfg.Location.RadiusKm,
			Interval: s.interval,
		}))
	}

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	g, ctx := errgroup.WithContext(ctx)

	if cfg.Server != nil && cfg.Server.Listen != "" {
		srv := server.New(&server.Options{
			Flights:  redisStore,
			Heard:    redisStore,
			Sources:  sourceNames(specs),
			Aircraft: pgStore,
			Routes:   pgStore,
			Alerts:   pgStore,
			Images:   hexdbClient,
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
		for _, n := range plannedNotifiers(cfg) {
			notifyMgr.Register(n.notifier)
			slog.InfoContext(ctx, "notifications enabled", slog.String("backend", n.name))
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

	for _, p := range pollers {
		g.Go(func() error { p.Run(ctx); return nil })
	}

	if err := g.Wait(); err != nil {
		slog.ErrorContext(ctx, "shutdown error", slog.String("error", err.Error()))
	}
	slog.InfoContext(ctx, "shutdown complete")
}

// setup parses the log level, starts observability, installs the trace-aware
// default logger, and loads the config. Every failure here is fatal: the
// service has nothing useful to do without them. The returned function flushes
// pending spans and must be deferred by the caller, not here, or the trace
// provider is torn down before anything has been traced.
func setup(ctx context.Context, configPath, logLevel string) (*config.Config, func()) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(logLevel)); err != nil {
		fatal(ctx, "invalid log level", err)
	}

	otelShutdown, err := observe.Setup(ctx, "flight-fetcher", Version)
	if err != nil {
		fatal(ctx, "failed to initialize observability", err)
	}
	shutdown := func() {
		if err := otelShutdown(ctx); err != nil {
			slog.ErrorContext(ctx, "observability shutdown error", slog.String("error", err.Error()))
		}
	}

	slog.SetDefault(slog.New(&observe.TracedHandler{
		Handler: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}),
	}))

	cfg, err := config.Load(configPath)
	if err != nil {
		shutdown()
		fatal(ctx, "failed to load config", err)
	}
	return cfg, shutdown
}

// fatal logs the error and exits non-zero. Used only for startup failures,
// where there is no degraded mode worth running in.
func fatal(ctx context.Context, msg string, err error) {
	slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
	os.Exit(1)
}
