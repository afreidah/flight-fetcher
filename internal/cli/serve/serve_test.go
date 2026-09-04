// -------------------------------------------------------------------------------
// Serve - Daemon Bootstrap Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Covers the startup order and the failure paths of Run. Because Run returns
// errors instead of exiting, each way startup can fail is assertable here
// rather than only observable as a non-zero exit status.
//
// A successful Run needs a live Redis and Postgres, so the happy path belongs
// to the integration suites. What is covered here is everything up to the
// point real infrastructure is required, plus the optional-component guards,
// which decide what runs at all and are pure functions of config.
// -------------------------------------------------------------------------------

package serve

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/afreidah/flight-fetcher/internal/config"

	"golang.org/x/sync/errgroup"
)

// -------------------------------------------------------------------------
// FIXTURES
// -------------------------------------------------------------------------

// validConfig is the smallest config that passes validation. The Redis and
// Postgres addresses point at a port nothing listens on, so a Run using it
// gets as far as the first connection attempt and then fails predictably.
const validConfig = `
location {
  lat       = 34.0928
  lon       = -118.3287
  radius_km = 50.0
}

opensky {
  id     = "test-client"
  secret = "test-secret"
}

poll_interval = "20s"

redis {
  addr = "127.0.0.1:1"
}

postgres {
  dsn = "postgres://user:pass@127.0.0.1:1/testdb?sslmode=disable"
}
`

// writeConfig writes HCL to a temp file and returns its path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.hcl")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	return path
}

// -------------------------------------------------------------------------
// STARTUP FAILURES
// -------------------------------------------------------------------------

// TestRun_InvalidLogLevel verifies an unparseable log level fails before any
// observability or network setup happens, and that the message names the
// offending value rather than just reporting a parse failure.
func TestRun_InvalidLogLevel(t *testing.T) {
	err := Run(context.Background(), Options{
		ConfigPath: writeConfig(t, validConfig),
		LogLevel:   "chatty",
		Version:    "test",
	})
	if err == nil {
		t.Fatal("Run() error = nil, want an invalid log level error")
	}
	if !strings.Contains(err.Error(), "invalid log level") {
		t.Errorf("error = %q, want it to mention the invalid log level", err)
	}
	if !strings.Contains(err.Error(), "chatty") {
		t.Errorf("error = %q, want it to name the offending value", err)
	}
}

// TestRun_MissingConfig verifies a config that cannot be loaded is reported as
// such. This path runs after observability is initialized, so it also
// exercises the shutdown-on-failure branch in setup.
func TestRun_MissingConfig(t *testing.T) {
	err := Run(context.Background(), Options{
		ConfigPath: filepath.Join(t.TempDir(), "nope.hcl"),
		LogLevel:   "error",
		Version:    "test",
	})
	if err == nil {
		t.Fatal("Run() error = nil, want a config load error")
	}
	if !strings.Contains(err.Error(), "loading config") {
		t.Errorf("error = %q, want it to mention loading config", err)
	}
}

// TestRun_RedisUnreachable verifies the first real dependency check fails with
// a message naming Redis. Postgres is equally unreachable in this config, so
// the assertion also pins the order: Redis is checked first, and a failure
// there stops startup before a Postgres pool is opened.
func TestRun_RedisUnreachable(t *testing.T) {
	// Bounded rather than open-ended: go-redis retries a refused dial with
	// backoff, which takes ~12s to give up on its own. The deadline proves the
	// same path in a fraction of that, since Ping honours the context.
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	err := Run(ctx, Options{
		ConfigPath: writeConfig(t, validConfig),
		LogLevel:   "error",
		Version:    "test",
	})
	if err == nil {
		t.Fatal("Run() error = nil, want a redis connection error")
	}
	if !strings.Contains(err.Error(), "connecting to redis") {
		t.Errorf("error = %q, want it to name redis", err)
	}
}

// -------------------------------------------------------------------------
// SETUP
// -------------------------------------------------------------------------

// TestSetup_ValidLevels verifies every level name the flag documents is
// accepted, so the help text and the parser cannot drift apart.
func TestSetup_ValidLevels(t *testing.T) {
	path := writeConfig(t, validConfig)

	for _, level := range []string{"debug", "info", "warn", "error"} {
		t.Run(level, func(t *testing.T) {
			cfg, shutdown, err := setup(context.Background(), Options{
				ConfigPath: path,
				LogLevel:   level,
				Version:    "test",
			})
			if err != nil {
				t.Fatalf("setup() error = %v", err)
			}
			defer shutdown()

			if cfg == nil {
				t.Fatal("setup() returned a nil config")
			}
			if cfg.Redis.Addr != "127.0.0.1:1" {
				t.Errorf("Redis.Addr = %q, want the value from the file", cfg.Redis.Addr)
			}
		})
	}
}

// TestSetup_ReturnsUsableShutdown verifies the returned shutdown is a real
// function and is safe to call. A constructor that returns a cleanup must
// return one that does something; returning nil here would panic the deferred
// call in Run instead of failing visibly.
func TestSetup_ReturnsUsableShutdown(t *testing.T) {
	cfg, shutdown, err := setup(context.Background(), Options{
		ConfigPath: writeConfig(t, validConfig),
		LogLevel:   "error",
		Version:    "test",
	})
	if err != nil {
		t.Fatalf("setup() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("setup() returned a nil config")
	}
	if shutdown == nil {
		t.Fatal("setup() returned a nil shutdown func")
	}
	shutdown()
}

// -------------------------------------------------------------------------
// POLLER ASSEMBLY
// -------------------------------------------------------------------------

// TestBuildPollers verifies one poller is constructed per planned source, in
// order. deps is left zero because poller.New only records its dependencies;
// nothing here starts a poll cycle.
func TestBuildPollers(t *testing.T) {
	cfg := &config.Config{
		Poll:           20 * time.Second,
		EnrichInterval: time.Hour,
		Location:       config.Location{Lat: 34.05, Lon: -118.25, RadiusKm: 50},
		OpenSky:        &config.OpenSkyConfig{},
		Dump1090:       &config.Dump1090Config{URL: "http://antenna.local", Interval: 5 * time.Second},
	}
	specs := plannedSources(cfg)
	if len(specs) != 2 {
		t.Fatalf("plannedSources() returned %d specs, want 2", len(specs))
	}

	got := buildPollers(t.Context(), cfg, specs, deps{})
	if len(got) != len(specs) {
		t.Fatalf("buildPollers() returned %d pollers, want %d", len(got), len(specs))
	}
	for i, p := range got {
		if p == nil {
			t.Errorf("poller[%d] is nil", i)
		}
	}
}

// TestBuildPollers_NoSources verifies an empty plan yields no pollers rather
// than a nil-length slice the caller has to guard.
func TestBuildPollers_NoSources(t *testing.T) {
	got := buildPollers(t.Context(), &config.Config{}, nil, deps{})
	if len(got) != 0 {
		t.Errorf("buildPollers() returned %d pollers, want 0", len(got))
	}
}

// -------------------------------------------------------------------------
// OPTIONAL COMPONENT GUARDS
// -------------------------------------------------------------------------

// TestStartServer_Optional verifies the dashboard is registered only when a
// listen address is configured. A config with a server block but no address is
// the case worth pinning: the block exists, so a naive nil check would start a
// server bound to nothing.
func TestStartServer_Optional(t *testing.T) {
	// Hold a port open so a server that does get registered fails immediately
	// at Listen with "address already in use". That failure is the evidence
	// registration happened, and it lands before any request could reach the
	// nil stores passed in here.
	ln, err := net.Listen("tcp", "127.0.0.1:0") //nolint:noctx // fixture listener, nothing to cancel
	if err != nil {
		t.Fatalf("reserving a port: %v", err)
	}
	defer ln.Close()

	tests := []struct {
		name           string
		server         *config.ServerConfig
		wantRegistered bool
	}{
		{"no server block", nil, false},
		{"server block with no listen address", &config.ServerConfig{}, false},
		{"listen address configured", &config.ServerConfig{Listen: ln.Addr().String()}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, gctx := errgroup.WithContext(t.Context())
			startServer(gctx, g, &config.Config{Server: tt.server}, "test", nil, deps{})
			err := g.Wait()

			if tt.wantRegistered && err == nil {
				t.Error("Wait() = nil, want the listen failure proving a server was registered")
			}
			if !tt.wantRegistered && err != nil {
				t.Errorf("Wait() = %v, want nil with nothing registered", err)
			}
		})
	}
}

// TestStartSquawkMonitor_Optional verifies the monitor is skipped entirely
// when unconfigured, so nothing is registered on the group.
func TestStartSquawkMonitor_Optional(t *testing.T) {
	g, gctx := errgroup.WithContext(t.Context())

	startSquawkMonitor(gctx, g, &config.Config{SquawkMonitor: nil}, deps{})

	if err := g.Wait(); err != nil {
		t.Errorf("Wait() error = %v, want nil with nothing registered", err)
	}
}

// TestStartRetention_Optional verifies the same for the retention sweeper.
func TestStartRetention_Optional(t *testing.T) {
	g, gctx := errgroup.WithContext(t.Context())

	startRetention(gctx, g, &config.Config{Retention: nil}, deps{})

	if err := g.Wait(); err != nil {
		t.Errorf("Wait() error = %v, want nil with nothing registered", err)
	}
}

// -------------------------------------------------------------------------
// SIGNAL HANDLING
// -------------------------------------------------------------------------

// TestSignalContext verifies the returned context is live until stopped, and
// that stopping it cancels. The signal delivery itself is the runtime's job;
// what matters here is that the caller gets a context it can shut down.
func TestSignalContext(t *testing.T) {
	ctx, stop := SignalContext(context.Background())

	select {
	case <-ctx.Done():
		t.Fatal("context cancelled before stop was called")
	default:
	}

	stop()

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Error("context not cancelled after stop")
	}
}
