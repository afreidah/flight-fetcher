-- name: UpsertAircraftMeta :exec
INSERT INTO aircraft_meta (icao24, registration, manufacturer, type, operator)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (icao24) DO UPDATE SET
    registration = EXCLUDED.registration,
    manufacturer = EXCLUDED.manufacturer,
    type = EXCLUDED.type,
    operator = EXCLUDED.operator;

-- name: GetAircraftMeta :one
SELECT icao24, registration, manufacturer, type, operator
FROM aircraft_meta
WHERE icao24 = $1;
