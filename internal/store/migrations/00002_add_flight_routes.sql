-- +goose Up
CREATE TABLE flight_routes (
    callsign    TEXT PRIMARY KEY,
    dep_iata    TEXT NOT NULL DEFAULT '',
    dep_icao    TEXT NOT NULL DEFAULT '',
    dep_name    TEXT NOT NULL DEFAULT '',
    arr_iata    TEXT NOT NULL DEFAULT '',
    arr_icao    TEXT NOT NULL DEFAULT '',
    arr_name    TEXT NOT NULL DEFAULT ''
);

-- +goose Down
DROP TABLE IF EXISTS flight_routes;
