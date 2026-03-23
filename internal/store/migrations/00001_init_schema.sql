-- +goose Up
CREATE TABLE aircraft_meta (
    icao24       TEXT PRIMARY KEY,
    registration TEXT NOT NULL DEFAULT '',
    manufacturer TEXT NOT NULL DEFAULT '',
    type         TEXT NOT NULL DEFAULT '',
    operator     TEXT NOT NULL DEFAULT ''
);

CREATE TABLE sightings (
    id          BIGSERIAL PRIMARY KEY,
    icao24      TEXT NOT NULL,
    lat         DOUBLE PRECISION NOT NULL,
    lon         DOUBLE PRECISION NOT NULL,
    distance_km DOUBLE PRECISION NOT NULL,
    seen_at     TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_sightings_icao24 ON sightings (icao24);
CREATE INDEX idx_sightings_seen_at ON sightings (seen_at);

-- +goose Down
DROP TABLE IF EXISTS sightings;
DROP TABLE IF EXISTS aircraft_meta;
