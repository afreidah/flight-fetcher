-- +goose Up
ALTER TABLE flight_routes ADD COLUMN cached_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- +goose Down
ALTER TABLE flight_routes DROP COLUMN cached_at;
