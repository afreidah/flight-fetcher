package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/afreidah/flight-fetcher/internal/config"
	"github.com/afreidah/flight-fetcher/internal/enricher"
	"github.com/afreidah/flight-fetcher/internal/geo"
	"github.com/afreidah/flight-fetcher/internal/hexdb"
	"github.com/afreidah/flight-fetcher/internal/opensky"
	"github.com/afreidah/flight-fetcher/internal/poller"
	"github.com/afreidah/flight-fetcher/internal/store"
)

func main() {
	configPath := flag.String("config", "config.hcl", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	pollInterval, err := cfg.PollDuration()
	if err != nil {
		log.Fatalf("parsing poll interval: %v", err)
	}

	// OpenSky credentials from environment (injected by Vault)
	oskyUser := os.Getenv("OPENSKY_USERNAME")
	oskyPass := os.Getenv("OPENSKY_PASSWORD")

	oskyClient := opensky.NewClient(oskyUser, oskyPass)
	hexdbClient := hexdb.NewClient()

	redisStore := store.NewRedisStore(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	defer redisStore.Close()

	pgStore, err := store.NewPostgresStore(cfg.Postgres.DSN)
	if err != nil {
		log.Fatalf("connecting to postgres: %v", err)
	}
	defer pgStore.Close()

	enr := enricher.New(hexdbClient, pgStore)

	center := geo.Coord{Lat: cfg.Location.Lat, Lon: cfg.Location.Lon}

	p := poller.New(oskyClient, redisStore, pgStore, enr, center, cfg.Location.RadiusKm, pollInterval)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	p.Run(ctx)
}
