-- name: UpsertFlightRoute :exec
INSERT INTO flight_routes (callsign, dep_iata, dep_icao, dep_name, arr_iata, arr_icao, arr_name, cached_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now())
ON CONFLICT (callsign) DO UPDATE SET
    dep_iata = EXCLUDED.dep_iata,
    dep_icao = EXCLUDED.dep_icao,
    dep_name = EXCLUDED.dep_name,
    arr_iata = EXCLUDED.arr_iata,
    arr_icao = EXCLUDED.arr_icao,
    arr_name = EXCLUDED.arr_name,
    cached_at = now();

-- name: GetFlightRoute :one
SELECT callsign, dep_iata, dep_icao, dep_name, arr_iata, arr_icao, arr_name
FROM flight_routes
WHERE callsign = $1 AND cached_at > $2;

-- name: DeleteOldRoutes :execresult
DELETE FROM flight_routes WHERE cached_at < $1;
