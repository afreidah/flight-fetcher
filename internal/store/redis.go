package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/afreidah/flight-fetcher/internal/opensky"
)

const flightKeyPrefix = "flight:"
const defaultTTL = 2 * time.Minute

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(addr, password string, db int) *RedisStore {
	return &RedisStore{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       db,
		}),
	}
}

// SetFlight stores the current state of a flight, keyed by ICAO24 with a TTL.
func (r *RedisStore) SetFlight(ctx context.Context, sv opensky.StateVector) error {
	data, err := json.Marshal(sv)
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}
	key := flightKeyPrefix + sv.ICAO24
	return r.client.Set(ctx, key, data, defaultTTL).Err()
}

// GetFlight retrieves the current state of a flight by ICAO24.
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

func (r *RedisStore) Close() error {
	return r.client.Close()
}
