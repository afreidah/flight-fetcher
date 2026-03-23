package enricher

import (
	"context"
	"log"

	"github.com/afreidah/flight-fetcher/internal/hexdb"
	"github.com/afreidah/flight-fetcher/internal/store"
)

type Enricher struct {
	hexdb    *hexdb.Client
	postgres *store.PostgresStore
}

func New(h *hexdb.Client, pg *store.PostgresStore) *Enricher {
	return &Enricher{hexdb: h, postgres: pg}
}

// Enrich looks up and caches aircraft metadata if not already known.
// Returns true if this is a newly seen aircraft.
func (e *Enricher) Enrich(ctx context.Context, icao24 string) bool {
	existing, err := e.postgres.GetAircraftMeta(ctx, icao24)
	if err != nil {
		log.Printf("error checking aircraft meta for %s: %v", icao24, err)
		return false
	}
	if existing != nil {
		return false
	}

	info, err := e.hexdb.Lookup(ctx, icao24)
	if err != nil {
		log.Printf("error looking up %s in hexdb: %v", icao24, err)
		return true
	}
	if info == nil {
		log.Printf("no hexdb data for %s", icao24)
		return true
	}

	if err := e.postgres.SaveAircraftMeta(ctx, info); err != nil {
		log.Printf("error saving aircraft meta for %s: %v", icao24, err)
	}
	return true
}
