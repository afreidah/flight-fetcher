-- name: UpsertAircraftMeta :exec
INSERT INTO aircraft_meta (icao24, registration, manufacturer, type, operator, updated_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (icao24) DO UPDATE SET
    registration = EXCLUDED.registration,
    manufacturer = EXCLUDED.manufacturer,
    type = EXCLUDED.type,
    operator = EXCLUDED.operator,
    updated_at = now();

-- name: GetAircraftMeta :one
SELECT icao24, registration, manufacturer, type, operator
FROM aircraft_meta
WHERE icao24 = $1;
