-- +goose Up

-- Composite index for HasRecentSquawkAlert query (icao24 + squawk + seen_at)
CREATE INDEX idx_squawk_alerts_lookup ON squawk_alerts (icao24, squawk, seen_at);

-- Composite index for sighting queries filtered by aircraft and time
CREATE INDEX idx_sightings_icao24_seen_at ON sightings (icao24, seen_at);

-- Track when aircraft metadata was last refreshed for cache staleness
ALTER TABLE aircraft_meta ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- +goose Down
DROP INDEX IF EXISTS idx_squawk_alerts_lookup;
DROP INDEX IF EXISTS idx_sightings_icao24_seen_at;
ALTER TABLE aircraft_meta DROP COLUMN IF EXISTS updated_at;
