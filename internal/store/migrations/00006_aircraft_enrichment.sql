-- +goose Up
ALTER TABLE aircraft_meta
    ADD COLUMN IF NOT EXISTS icao_type_code TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS registered_owners TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS image_url TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE aircraft_meta
    DROP COLUMN IF EXISTS icao_type_code,
    DROP COLUMN IF EXISTS registered_owners,
    DROP COLUMN IF EXISTS image_url;
