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
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/afreidah/flight-fetcher/internal/apiclient/opensky"

	"github.com/redis/go-redis/extra/redisotel/v9"
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
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 2,
	})

	ctx := context.Background()
	if err := redisotel.InstrumentTracing(client); err != nil {
		slog.WarnContext(ctx, "failed to instrument redis tracing", slog.String("error", err.Error()))
	}
	if err := redisotel.InstrumentMetrics(client); err != nil {
		slog.WarnContext(ctx, "failed to instrument redis metrics", slog.String("error", err.Error()))
	}

	return &RedisStore{client: client, ttl: ttl}
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
// to collect keys without blocking Redis, then MGET to fetch all values in
// a single round trip.
func (r *RedisStore) GetAllFlights(ctx context.Context) ([]opensky.StateVector, error) {
	var keys []string
	iter := r.client.Scan(ctx, 0, flightKeyPrefix+"*", 0).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return []opensky.StateVector{}, nil
	}

	vals, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("mget flights: %w", err)
	}

	flights := make([]opensky.StateVector, 0, len(vals))
	for i, val := range vals {
		str, ok := val.(string)
		if !ok {
			slog.WarnContext(ctx, "skipping nil flight key",
				slog.String("key", keys[i]))
			continue
		}
		var sv opensky.StateVector
		if err := json.Unmarshal([]byte(str), &sv); err != nil {
			slog.WarnContext(ctx, "skipping malformed flight data",
				slog.String("key", keys[i]),
				slog.String("error", err.Error()))
			continue
		}
		flights = append(flights, sv)
	}
	return flights, nil
}

// GetFlight retrieves the current state of a flight by ICAO24. Returns nil
// if the flight is not in Redis (expired or never seen).
func (r *RedisStore) GetFlight(ctx context.Context, icao24 string) (*opensky.StateVector, error) {
	key := flightKeyPrefix + icao24
	data, err := r.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
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

// Ping verifies the Redis connection is alive.
func (r *RedisStore) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close shuts down the Redis client connection.
func (r *RedisStore) Close() error {
	return r.client.Close()
}
