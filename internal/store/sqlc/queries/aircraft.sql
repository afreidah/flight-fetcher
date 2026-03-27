-- name: UpsertAircraftMeta :exec
INSERT INTO aircraft_meta (icao24, registration, manufacturer, type, operator, icao_type_code, registered_owners, image_url, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
ON CONFLICT (icao24) DO UPDATE SET
    registration = EXCLUDED.registration,
    manufacturer = EXCLUDED.manufacturer,
    type = EXCLUDED.type,
    operator = EXCLUDED.operator,
    icao_type_code = EXCLUDED.icao_type_code,
    registered_owners = EXCLUDED.registered_owners,
    image_url = EXCLUDED.image_url,
    updated_at = now();

-- name: GetAircraftMeta :one
SELECT icao24, registration, manufacturer, type, operator, icao_type_code, registered_owners, image_url
FROM aircraft_meta
WHERE icao24 = $1 AND updated_at > $2;
