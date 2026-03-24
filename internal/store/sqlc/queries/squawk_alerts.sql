-- name: InsertSquawkAlert :exec
INSERT INTO squawk_alerts (icao24, callsign, squawk, lat, lon, seen_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: HasRecentSquawkAlert :one
SELECT EXISTS(
  SELECT 1 FROM squawk_alerts
  WHERE icao24 = $1 AND squawk = $2 AND seen_at > $3
) AS exists;

-- name: GetRecentSquawkAlerts :many
SELECT id, icao24, callsign, squawk, lat, lon, seen_at
FROM squawk_alerts
WHERE seen_at > $1
ORDER BY seen_at DESC;

-- name: DeleteOldSquawkAlerts :execresult
DELETE FROM squawk_alerts WHERE seen_at < $1;
