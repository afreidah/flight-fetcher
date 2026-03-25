-- name: LogSighting :exec
INSERT INTO sightings (icao24, lat, lon, distance_km, seen_at)
VALUES ($1, $2, $3, $4, $5);

-- name: DeleteOldSightings :execresult
DELETE FROM sightings WHERE id IN (
    SELECT s.id FROM sightings s WHERE s.seen_at < $1 LIMIT 10000
);
