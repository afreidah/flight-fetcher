-- name: LogSighting :exec
INSERT INTO sightings (icao24, lat, lon, distance_km, seen_at)
VALUES ($1, $2, $3, $4, $5);

-- name: DeleteOldSightings :execresult
DELETE FROM sightings WHERE seen_at < $1;
