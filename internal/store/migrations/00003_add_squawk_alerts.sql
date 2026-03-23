-- +goose Up
CREATE TABLE squawk_alerts (
    id          BIGSERIAL PRIMARY KEY,
    icao24      TEXT NOT NULL,
    callsign    TEXT NOT NULL DEFAULT '',
    squawk      TEXT NOT NULL,
    lat         DOUBLE PRECISION NOT NULL,
    lon         DOUBLE PRECISION NOT NULL,
    seen_at     TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_squawk_alerts_seen_at ON squawk_alerts (seen_at);
CREATE INDEX idx_squawk_alerts_squawk ON squawk_alerts (squawk);

-- +goose Down
DROP TABLE IF EXISTS squawk_alerts;
