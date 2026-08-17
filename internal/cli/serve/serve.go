// -------------------------------------------------------------------------------
// Serve - Flight Fetcher Daemon Bootstrap
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// The composition root: initializes observability, loads configuration, opens
// the Redis and Postgres stores, assembles the polling pipeline, and runs every
// long-lived component under one errgroup until the context is cancelled.
//
// Run returns errors rather than calling os.Exit, which is what keeps this
// package testable. cmd/server is a shim over flag parsing and signal handling
// that translates a returned error into an exit status; everything with
// behaviour worth covering lives here or in wiring.go.
// -------------------------------------------------------------------------------

// Package serve implements the flight-fetcher daemon. It owns the startup
// order, dependency construction, and shutdown of every long-lived component.
// The decisions about what to construct are pure functions in wiring.go.
package serve

import (
	"context"
	"fmt"
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

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// Options configures a daemon run. Version is stamped into the binary at build
// time and reported by the dashboard and the trace resource, so it is passed
// down from the entrypoint rather than read from a package variable here.
type Options struct {
	ConfigPath string
	LogLevel   string
	Version    string
}

// deps are the long-lived dependencies Run constructs and the components
// share. They travel as one value because nearly every start helper needs some
// subset of them, and threading them individually pushed startServer past the
// parameter limit the style guide sets for exactly this reason.
type deps struct {
	cache    *store.RedisStore
	db       *store.PostgresStore
	images   *hexdb.Client
	enricher *enricher.Enricher
}

// Run starts the daemon and blocks until ctx is cancelled or a component
// fails. Every startup failure is returned rather than exiting, so a caller
// can decide what to do with it and a test can assert on it.
//
// The caller owns signal handling: Run treats ctx cancellation as the shutdown
// signal and returns once every component has stopped.
func Run(ctx context.Context, opts Options) error {
	cfg, shutdownObs, err := setup(ctx, opts)
	if err != nil {
		return err
	}
	defer shutdownObs()

	specs := plannedSources(cfg)
	slowest, redisTTL := cacheTTL(specs)
	slog.InfoContext(ctx, "redis ttl computed from slowest poller",
		slog.Duration("slowest_interval", slowest),
		slog.Duration("redis_ttl", redisTTL))

	redisStore := store.NewRedisStore(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB, redisTTL)
	defer redisStore.Close()
	if err := redisStore.Ping(ctx); err != nil {
		return fmt.Errorf("connecting to redis: %w", err)
	}

	pgStore, err := store.NewPostgresStore(ctx, cfg.Postgres.DSN, 0)
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	defer pgStore.Close()

	hexdbClient := hexdb.NewClient()
	routeSources := plannedRouteSources(cfg)
	for _, s := range routeSources {
		slog.InfoContext(ctx, "route enrichment enabled", slog.String("source", s.Name))
	}

	d := deps{
		cache:  redisStore,
		db:     pgStore,
		images: hexdbClient,
		enricher: enricher.New(&enricher.Options{
			AircraftSources: plannedAircraftSources(cfg, hexdbClient),
			Store:           pgStore,
			RouteSources:    routeSources,
			RouteStore:      pgStore,
		}),
	}

	pollers := buildPollers(ctx, cfg, specs, d)

	g, ctx := errgroup.WithContext(ctx)
	startServer(ctx, g, cfg, opts.Version, specs, d)
	startSquawkMonitor(ctx, g, cfg, d)
	startRetention(ctx, g, cfg, d)
	for _, p := range pollers {
		g.Go(func() error { p.Run(ctx); return nil })
	}

	if err := g.Wait(); err != nil {
		slog.ErrorContext(ctx, "shutdown error", slog.String("error", err.Error()))
		return err
	}
	slog.InfoContext(ctx, "shutdown complete")
	return nil
}

// -------------------------------------------------------------------------
// INTERNALS
// -------------------------------------------------------------------------

// setup parses the log level, starts observability, installs the trace-aware
// default logger, and loads the config. The returned function flushes pending
// spans and must be deferred by the caller, not here, or the trace provider is
// torn down before anything has been traced.
func setup(ctx context.Context, opts Options) (*config.Config, func(), error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(opts.LogLevel)); err != nil {
		return nil, nil, fmt.Errorf("invalid log level %q: %w", opts.LogLevel, err)
	}

	otelShutdown, err := observe.Setup(ctx, "flight-fetcher", opts.Version)
	if err != nil {
		return nil, nil, fmt.Errorf("initializing observability: %w", err)
	}
	shutdown := func() {
		if err := otelShutdown(ctx); err != nil {
			slog.ErrorContext(ctx, "observability shutdown error", slog.String("error", err.Error()))
		}
	}

	slog.SetDefault(slog.New(&observe.TracedHandler{
		Handler: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}),
	}))

	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		shutdown()
		return nil, nil, fmt.Errorf("loading config: %w", err)
	}
	return cfg, shutdown, nil
}

// buildPollers constructs one poller per planned source, all sharing the dedup
// state so enrichment is not duplicated when two sources hear the same
// aircraft.
func buildPollers(ctx context.Context, cfg *config.Config, specs []sourceSpec, d deps) []*poller.Poller {
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
			Cache:    d.cache,
			Logger:   d.db,
			Enricher: d.enricher,
			Dedup:    dedup,
			Center:   center,
			RadiusKm: cfg.Location.RadiusKm,
			Interval: s.interval,
		}))
	}
	return pollers
}

// startServer registers the dashboard HTTP server on g when a listen address
// is configured. Without one the service runs headless, polling and storing
// but serving nothing.
func startServer(ctx context.Context, g *errgroup.Group, cfg *config.Config, version string, specs []sourceSpec, d deps) {
	if cfg.Server == nil || cfg.Server.Listen == "" {
		return
	}
	srv := server.New(&server.Options{
		Flights:  d.cache,
		Heard:    d.cache,
		Sources:  sourceNames(specs),
		Aircraft: d.db,
		Routes:   d.db,
		Alerts:   d.db,
		Images:   d.images,
		Pingers: []server.HealthPinger{
			{Name: "redis", Pinger: d.cache},
			{Name: "postgres", Pinger: d.db},
		},
		Version:    version,
		RefreshSec: cfg.Server.RefreshSeconds(),
	})
	g.Go(func() error { return srv.ListenAndServe(ctx, cfg.Server.Listen) })
}

// startSquawkMonitor registers the emergency squawk monitor on g when it is
// configured, wiring up whichever notification backends the config names. With
// none configured the manager is a no-op, so detection and storage still run
// without alerting.
func startSquawkMonitor(ctx context.Context, g *errgroup.Group, cfg *config.Config, d deps) {
	if cfg.SquawkMonitor == nil {
		return
	}
	notifyMgr := notify.NewManager()
	for _, n := range plannedNotifiers(cfg) {
		notifyMgr.Register(n.notifier)
		slog.InfoContext(ctx, "notifications enabled", slog.String("backend", n.name))
	}
	squawkClient := opensky.NewClient(cfg.OpenSky.ID, cfg.OpenSky.Secret)
	sm := squawk.New(squawkClient, d.db, d.enricher, notifyMgr, cfg.SquawkMonitor.Poll)
	g.Go(func() error { sm.Run(ctx); return nil })
}

// startRetention registers the retention sweeper on g when it is configured.
func startRetention(ctx context.Context, g *errgroup.Group, cfg *config.Config, d deps) {
	if cfg.Retention == nil {
		return
	}
	r := cfg.Retention
	rw := retention.New(d.db, r.Sightings, r.Alerts, r.Routes, r.CleanInterval)
	g.Go(func() error { rw.Run(ctx); return nil })
}

// SignalContext returns a context cancelled on SIGINT or SIGTERM. It lives
// here rather than in the entrypoint so the shutdown signals the daemon
// responds to are recorded alongside the daemon itself.
func SignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
}
