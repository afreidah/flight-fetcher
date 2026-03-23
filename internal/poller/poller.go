package poller

import (
	"context"
	"log"
	"time"

	"github.com/afreidah/flight-fetcher/internal/enricher"
	"github.com/afreidah/flight-fetcher/internal/geo"
	"github.com/afreidah/flight-fetcher/internal/opensky"
	"github.com/afreidah/flight-fetcher/internal/store"
)

type Poller struct {
	opensky  *opensky.Client
	redis    *store.RedisStore
	postgres *store.PostgresStore
	enricher *enricher.Enricher
	center   geo.Coord
	radiusKm float64
	interval time.Duration
}

func New(
	osky *opensky.Client,
	redis *store.RedisStore,
	pg *store.PostgresStore,
	enr *enricher.Enricher,
	center geo.Coord,
	radiusKm float64,
	interval time.Duration,
) *Poller {
	return &Poller{
		opensky:  osky,
		redis:    redis,
		postgres: pg,
		enricher: enr,
		center:   center,
		radiusKm: radiusKm,
		interval: interval,
	}
}

// Run starts the polling loop. It blocks until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	log.Printf("poller started: center=(%f, %f) radius=%.1fkm interval=%s",
		p.center.Lat, p.center.Lon, p.radiusKm, p.interval)

	p.poll(ctx)
	for {
		select {
		case <-ctx.Done():
			log.Println("poller stopped")
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

func (p *Poller) poll(ctx context.Context) {
	bbox := geo.BBoxAround(p.center, p.radiusKm)
	resp, err := p.opensky.GetStates(ctx, bbox)
	if err != nil {
		log.Printf("poll error: %v", err)
		return
	}

	count := 0
	for _, sv := range resp.States {
		dist := geo.HaversineKm(p.center, geo.Coord{Lat: sv.Latitude, Lon: sv.Longitude})
		if dist > p.radiusKm {
			continue
		}

		if err := p.redis.SetFlight(ctx, sv); err != nil {
			log.Printf("redis error for %s: %v", sv.ICAO24, err)
		}

		if err := p.postgres.LogSighting(ctx, sv.ICAO24, sv.Latitude, sv.Longitude, dist); err != nil {
			log.Printf("sighting log error for %s: %v", sv.ICAO24, err)
		}

		p.enricher.Enrich(ctx, sv.ICAO24)
		count++
	}
	log.Printf("poll complete: %d aircraft within %.1fkm", count, p.radiusKm)
}
