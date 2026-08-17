// -------------------------------------------------------------------------------
// Serve - Daemon Bootstrap Integration Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Drives Run end to end against real Redis and Postgres containers: load
// config, open both stores, assemble the pipeline, start every optional
// component, then shut down on context cancellation.
//
// The unit suite in serve_test.go covers the paths that fail before real
// infrastructure is needed. What is only reachable here is the successful
// startup itself, which is most of Run and every configured branch of the
// start helpers. Skipped with -short.
// -------------------------------------------------------------------------------

package serve

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	redisModule "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

// -------------------------------------------------------------------------
// TEST SETUP
// -------------------------------------------------------------------------

// testDSN holds the DSN for the test Postgres container.
var testDSN string

// testRedisAddr holds the host:port for the test Redis container.
var testRedisAddr string

// TestMain starts Postgres and Redis once for the package. Under -short it
// starts nothing, so the unit suite still runs without Docker.
func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		os.Exit(m.Run())
	}

	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("flight_fetcher_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start postgres container: %v\n", err)
		os.Exit(1)
	}

	testDSN, err = pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get postgres connection string: %v\n", err)
		os.Exit(1)
	}

	redisContainer, err := redisModule.Run(ctx, "redis:7-alpine")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start redis container: %v\n", err)
		os.Exit(1)
	}

	// testcontainers returns "redis://host:port"; NewRedisStore wants host:port.
	raw, err := redisContainer.ConnectionString(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get redis connection string: %v\n", err)
		os.Exit(1)
	}
	testRedisAddr = strings.TrimSuffix(strings.TrimPrefix(raw, "redis://"), "/0")

	code := m.Run()

	_ = pgContainer.Terminate(ctx)
	_ = redisContainer.Terminate(ctx)
	os.Exit(code)
}

// -------------------------------------------------------------------------
// HELPERS
// -------------------------------------------------------------------------

// skipShort skips the test when -short is set, matching the store suites.
func skipShort(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
}

// fullConfig returns HCL enabling every optional component, so one Run covers
// each branch of the start helpers. The upstream credentials are deliberately
// fake and the antenna URL points nowhere: those calls fail, get logged, and
// the components keep running, which is the behaviour under test. The poll
// intervals are long enough that only the immediate first cycle fires.
func fullConfig(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf(`
location {
  lat       = 34.0928
  lon       = -118.3287
  radius_km = 50.0
}

opensky {
  id     = "test-client"
  secret = "test-secret"
}

poll_interval = "1h"

redis {
  addr = %q
}

postgres {
  dsn = %q
}

server {
  listen = "127.0.0.1:0"
}

dump1090 {
  url = "http://127.0.0.1:1/data/aircraft.json"
}

squawk_monitor {
  interval = "1h"
}

retention {
  sightings_max_age = "168h"
  alerts_max_age    = "720h"
  routes_max_age    = "24h"
  interval          = "1h"
}

notifications {
  discord {
    webhook_url = "https://discord.example/hook"
  }
}
`, testRedisAddr, testDSN)
}

// -------------------------------------------------------------------------
// END TO END
// -------------------------------------------------------------------------

// TestRun_StartsAndShutsDown drives the full startup path and then cancels.
// Run must return nil: a clean shutdown is not an error, and returning one
// would make the binary exit non-zero on every SIGTERM.
func TestRun_StartsAndShutsDown(t *testing.T) {
	skipShort(t)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)

	go func() {
		done <- Run(ctx, Options{
			ConfigPath: writeConfig(t, fullConfig(t)),
			LogLevel:   "error",
			Version:    "integration-test",
		})
	}()

	// Let startup complete and every component run its first cycle before
	// asking for shutdown; cancelling too early would not prove they started.
	time.Sleep(3 * time.Second)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run() = %v, want nil on a clean shutdown", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Run() did not return within 30s of cancellation")
	}
}

// TestRun_MinimalConfig drives the other side of every optional branch: no
// server, no squawk monitor, no retention, no antenna. The service still comes
// up and polls, which is the headless mode the guards exist to allow.
func TestRun_MinimalConfig(t *testing.T) {
	skipShort(t)

	minimal := fmt.Sprintf(`
location {
  lat       = 34.0928
  lon       = -118.3287
  radius_km = 50.0
}

opensky {
  id     = "test-client"
  secret = "test-secret"
}

poll_interval = "1h"

redis {
  addr = %q
}

postgres {
  dsn = %q
}
`, testRedisAddr, testDSN)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)

	go func() {
		done <- Run(ctx, Options{
			ConfigPath: writeConfig(t, minimal),
			LogLevel:   "error",
			Version:    "integration-test",
		})
	}()

	time.Sleep(2 * time.Second)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run() = %v, want nil on a clean shutdown", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Run() did not return within 30s of cancellation")
	}
}

// TestRun_PostgresUnreachable verifies startup stops at Postgres when Redis is
// reachable but the database is not. The unit suite covers the Redis failure;
// this is the branch after it, which needs a live Redis to reach at all.
func TestRun_PostgresUnreachable(t *testing.T) {
	skipShort(t)

	cfg := fmt.Sprintf(`
location {
  lat       = 34.0928
  lon       = -118.3287
  radius_km = 50.0
}

opensky {
  id     = "test-client"
  secret = "test-secret"
}

poll_interval = "1h"

redis {
  addr = %q
}

postgres {
  dsn = "postgres://nobody:nobody@127.0.0.1:1/nothing?sslmode=disable"
}
`, testRedisAddr)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	err := Run(ctx, Options{
		ConfigPath: writeConfig(t, cfg),
		LogLevel:   "error",
		Version:    "integration-test",
	})
	if err == nil {
		t.Fatal("Run() = nil, want a postgres connection error")
	}
	if !strings.Contains(err.Error(), "connecting to postgres") {
		t.Errorf("error = %q, want it to name postgres", err)
	}
}
