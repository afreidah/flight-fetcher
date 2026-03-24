// -------------------------------------------------------------------------------
// Store - Redis Current Flight State
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Manages ephemeral aircraft position data in Redis. Each flight is keyed by
// ICAO24 with a TTL so entries auto-expire when an aircraft leaves the area
// or stops broadcasting.
// -------------------------------------------------------------------------------

package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/afreidah/flight-fetcher/internal/opensky"

	"github.com/redis/go-redis/v9"
)

// -------------------------------------------------------------------------
// CONSTANTS
// -------------------------------------------------------------------------

// flightKeyPrefix is the Redis key prefix for flight state entries.
const flightKeyPrefix = "flight:"

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// RedisStore manages current flight state in Redis.
type RedisStore struct {
	client *redis.Client
	ttl    time.Duration
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// NewRedisStore creates a RedisStore connected to the given Redis instance.
// The ttl parameter controls how long flight entries persist before expiring.
func NewRedisStore(addr, password string, db int, ttl time.Duration) *RedisStore {
	return &RedisStore{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       db,
		}),
		ttl: ttl,
	}
}

// SetFlight stores the current state of a flight, keyed by ICAO24 with a TTL.
func (r *RedisStore) SetFlight(ctx context.Context, sv *opensky.StateVector) error {
	data, err := json.Marshal(sv)
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}
	key := flightKeyPrefix + sv.ICAO24
	return r.client.Set(ctx, key, data, r.ttl).Err()
}

// GetAllFlights returns all current flight states stored in Redis. Uses SCAN
// instead of KEYS to avoid blocking Redis during enumeration.
func (r *RedisStore) GetAllFlights(ctx context.Context) ([]opensky.StateVector, error) {
	var flights []opensky.StateVector
	iter := r.client.Scan(ctx, 0, flightKeyPrefix+"*", 0).Iterator()
	for iter.Next(ctx) {
		data, err := r.client.Get(ctx, iter.Val()).Bytes()
		if err != nil {
			continue
		}
		var sv opensky.StateVector
		if err := json.Unmarshal(data, &sv); err != nil {
			continue
		}
		flights = append(flights, sv)
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return flights, nil
}

// GetFlight retrieves the current state of a flight by ICAO24. Returns nil
// if the flight is not in Redis (expired or never seen).
func (r *RedisStore) GetFlight(ctx context.Context, icao24 string) (*opensky.StateVector, error) {
	key := flightKeyPrefix + icao24
	data, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var sv opensky.StateVector
	if err := json.Unmarshal(data, &sv); err != nil {
		return nil, err
	}
	return &sv, nil
}

// Close shuts down the Redis client connection.
func (r *RedisStore) Close() error {
	return r.client.Close()
}
