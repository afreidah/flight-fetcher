// -------------------------------------------------------------------------------
// Config - Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests HCL configuration loading, validation, and error handling for missing
// files and malformed input.
// -------------------------------------------------------------------------------

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLoad_ValidConfig verifies that a complete HCL config file is parsed correctly.
func TestLoad_ValidConfig(t *testing.T) {
	content := `
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
  addr = "localhost:6379"
}

postgres {
  dsn = "postgres://user:pass@localhost:5432/testdb?sslmode=disable"
}
`
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Location.Lat != 34.0928 {
		t.Errorf("Lat = %f, want 34.0928", cfg.Location.Lat)
	}
	if cfg.Location.Lon != -118.3287 {
		t.Errorf("Lon = %f, want -118.3287", cfg.Location.Lon)
	}
	if cfg.Location.RadiusKm != 50.0 {
		t.Errorf("RadiusKm = %f, want 50.0", cfg.Location.RadiusKm)
	}
	if cfg.OpenSky.ID != "test-client" {
		t.Errorf("OpenSky.ID = %q, want %q", cfg.OpenSky.ID, "test-client")
	}
	if cfg.OpenSky.Secret != "test-secret" {
		t.Errorf("OpenSky.Secret = %q, want %q", cfg.OpenSky.Secret, "test-secret")
	}
	if cfg.Poll != 20*time.Second {
		t.Errorf("Poll = %v, want 20s", cfg.Poll)
	}
	if cfg.Redis.Addr != "localhost:6379" {
		t.Errorf("Redis.Addr = %q, want %q", cfg.Redis.Addr, "localhost:6379")
	}
	if cfg.Postgres.DSN != "postgres://user:pass@localhost:5432/testdb?sslmode=disable" {
		t.Errorf("Postgres.DSN = %q", cfg.Postgres.DSN)
	}
	if cfg.Server != nil {
		t.Error("Server should be nil when block is omitted")
	}
	if cfg.AirLabs != nil {
		t.Error("AirLabs should be nil when block is omitted")
	}
	if cfg.SquawkMonitor != nil {
		t.Error("SquawkMonitor should be nil when block is omitted")
	}
	if cfg.Retention != nil {
		t.Error("Retention should be nil when block is omitted")
	}
}

// TestLoad_WithRetentionBlock verifies that the optional retention block is parsed correctly.
func TestLoad_WithRetentionBlock(t *testing.T) {
	content := `
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
  addr = "localhost:6379"
}

postgres {
  dsn = "postgres://user:pass@localhost:5432/testdb?sslmode=disable"
}

retention {
  sightings_max_age = "720h"
  alerts_max_age    = "168h"
  interval          = "2h"
}
`
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Retention == nil {
		t.Fatal("Retention should not be nil when block is present")
	}
	if cfg.Retention.Sightings != 720*time.Hour {
		t.Errorf("sightings = %v, want 720h", cfg.Retention.Sightings)
	}
	if cfg.Retention.Alerts != 168*time.Hour {
		t.Errorf("alerts = %v, want 168h", cfg.Retention.Alerts)
	}
	if cfg.Retention.CleanInterval != 2*time.Hour {
		t.Errorf("interval = %v, want 2h", cfg.Retention.CleanInterval)
	}
}

// TestLoad_WithRetentionBlock_DefaultInterval verifies that interval defaults to 1h.
func TestLoad_WithRetentionBlock_DefaultInterval(t *testing.T) {
	content := `
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
  addr = "localhost:6379"
}

postgres {
  dsn = "postgres://user:pass@localhost:5432/testdb?sslmode=disable"
}

retention {
  sightings_max_age = "720h"
  alerts_max_age    = "168h"
}
`
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Retention.CleanInterval != time.Hour {
		t.Errorf("interval = %v, want 1h (default)", cfg.Retention.CleanInterval)
	}
}

// TestLoad_WithSquawkMonitorBlock verifies that the optional squawk_monitor block is parsed correctly.
func TestLoad_WithSquawkMonitorBlock(t *testing.T) {
	content := `
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
  addr = "localhost:6379"
}

postgres {
  dsn = "postgres://user:pass@localhost:5432/testdb?sslmode=disable"
}

squawk_monitor {
  interval = "60s"
}
`
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SquawkMonitor == nil {
		t.Fatal("SquawkMonitor should not be nil when block is present")
	}
	if cfg.SquawkMonitor.Poll != 60*time.Second {
		t.Errorf("SquawkMonitor.Poll = %v, want 60s", cfg.SquawkMonitor.Poll)
	}
}

// TestLoad_WithServerBlock verifies that the optional server block is parsed correctly.
func TestLoad_WithServerBlock(t *testing.T) {
	content := `
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
  addr = "localhost:6379"
}

postgres {
  dsn = "postgres://user:pass@localhost:5432/testdb?sslmode=disable"
}

server {
  listen = ":8080"
}
`
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server == nil {
		t.Fatal("Server should not be nil when block is present")
	}
	if cfg.Server.Listen != ":8080" {
		t.Errorf("Server.Listen = %q, want %q", cfg.Server.Listen, ":8080")
	}
}

// TestLoad_WithAirLabsBlock verifies that the optional airlabs block is parsed correctly.
func TestLoad_WithAirLabsBlock(t *testing.T) {
	content := `
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
  addr = "localhost:6379"
}

postgres {
  dsn = "postgres://user:pass@localhost:5432/testdb?sslmode=disable"
}

airlabs {
  api_key = "test-key-123"
}
`
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AirLabs == nil {
		t.Fatal("AirLabs should not be nil when block is present")
	}
	if cfg.AirLabs.APIKey != "test-key-123" {
		t.Errorf("AirLabs.APIKey = %q, want %q", cfg.AirLabs.APIKey, "test-key-123")
	}
}

// TestLoad_MissingFile verifies that loading a nonexistent file returns an error.
func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/config.hcl")
	if err == nil {
		t.Error("Load() expected error for missing file, got nil")
	}
}

// TestLoad_InvalidHCL verifies that malformed HCL returns an error.
func TestLoad_InvalidHCL(t *testing.T) {
	path := writeTemp(t, "this is not valid { hcl }")
	_, err := Load(path)
	if err == nil {
		t.Error("Load() expected error for invalid HCL, got nil")
	}
}

// TestPollDuration_ParsedByLoad verifies that Load parses poll_interval into the Poll field.
func TestPollDuration_ParsedByLoad(t *testing.T) {
	content := `
poll_interval = "30s"
location {
  lat       = 0.0
  lon       = 0.0
  radius_km = 50.0
}
opensky {
  id     = "t"
  secret = "t"
}
redis { addr = "localhost:6379" }
postgres { dsn = "postgres://localhost/test" }
`
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Poll != 30*time.Second {
		t.Errorf("Poll = %v, want 30s", cfg.Poll)
	}
}

// TestRefreshSeconds_Default verifies that RefreshSeconds returns 5 when not set.
func TestRefreshSeconds_Default(t *testing.T) {
	cfg := &ServerConfig{}
	if got := cfg.RefreshSeconds(); got != 5 {
		t.Errorf("RefreshSeconds() = %d, want 5", got)
	}
}

// TestRefreshSeconds_Custom verifies that RefreshSeconds returns the configured value.
func TestRefreshSeconds_Custom(t *testing.T) {
	cfg := &ServerConfig{Refresh: 10}
	if got := cfg.RefreshSeconds(); got != 10 {
		t.Errorf("RefreshSeconds() = %d, want 10", got)
	}
}

// TestValidation_InvalidConfigs verifies that bad config values produce clear errors.
func TestValidation_InvalidConfigs(t *testing.T) {
	validBase := func(overrides string) string {
		return `
location {
  lat       = 0.0
  lon       = 0.0
  radius_km = 50.0
}
opensky {
  id     = "test"
  secret = "test"
}
poll_interval = "20s"
redis {
  addr = "localhost:6379"
}
postgres {
  dsn = "postgres://localhost/test"
}
` + overrides
	}

	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{
			name: "invalid latitude",
			config: `
location {
  lat       = 100.0
  lon       = 0.0
  radius_km = 50.0
}
opensky {
  id     = "test"
  secret = "test"
}
poll_interval = "20s"
redis { addr = "localhost:6379" }
postgres { dsn = "postgres://localhost/test" }
`,
			wantErr: "location.lat",
		},
		{
			name: "invalid longitude",
			config: `
location {
  lat       = 0.0
  lon       = 200.0
  radius_km = 50.0
}
opensky {
  id     = "test"
  secret = "test"
}
poll_interval = "20s"
redis { addr = "localhost:6379" }
postgres { dsn = "postgres://localhost/test" }
`,
			wantErr: "location.lon",
		},
		{
			name: "zero radius",
			config: `
location {
  lat       = 0.0
  lon       = 0.0
  radius_km = 0.0
}
opensky {
  id     = "test"
  secret = "test"
}
poll_interval = "20s"
redis { addr = "localhost:6379" }
postgres { dsn = "postgres://localhost/test" }
`,
			wantErr: "radius_km",
		},
		{
			name: "empty opensky id",
			config: `
location {
  lat       = 0.0
  lon       = 0.0
  radius_km = 50.0
}
opensky {
  id     = ""
  secret = "test"
}
poll_interval = "20s"
redis { addr = "localhost:6379" }
postgres { dsn = "postgres://localhost/test" }
`,
			wantErr: "opensky.id",
		},
		{
			name: "empty redis addr",
			config: `
location {
  lat       = 0.0
  lon       = 0.0
  radius_km = 50.0
}
opensky {
  id     = "test"
  secret = "test"
}
poll_interval = "20s"
redis { addr = "" }
postgres { dsn = "postgres://localhost/test" }
`,
			wantErr: "redis.addr",
		},
		{
			name: "empty postgres dsn",
			config: `
location {
  lat       = 0.0
  lon       = 0.0
  radius_km = 50.0
}
opensky {
  id     = "test"
  secret = "test"
}
poll_interval = "20s"
redis { addr = "localhost:6379" }
postgres { dsn = "" }
`,
			wantErr: "postgres.dsn",
		},
		{
			name:    "empty airlabs key",
			config:  validBase(`airlabs { api_key = "" }`),
			wantErr: "airlabs.api_key",
		},
		{
			name:    "empty flightaware key",
			config:  validBase(`flightaware { api_key = "" }`),
			wantErr: "flightaware.api_key",
		},
		{
			name: "empty discord webhook url",
			config: validBase(`notifications {
  discord {
    webhook_url = ""
  }
}`),
			wantErr: "notifications.discord[0].webhook_url",
		},
		{
			name: "empty telegram bot token",
			config: validBase(`notifications {
  telegram {
    bot_token = ""
    chat_id   = "123"
  }
}`),
			wantErr: "notifications.telegram[0].bot_token",
		},
		{
			name: "empty telegram chat id",
			config: validBase(`notifications {
  telegram {
    bot_token = "tok"
    chat_id   = ""
  }
}`),
			wantErr: "notifications.telegram[0].chat_id",
		},
		{
			name:    "empty dump1090 url",
			config:  validBase(`dump1090 { url = "" }`),
			wantErr: "dump1090.url",
		},
		{
			name:    "invalid squawk monitor interval",
			config:  validBase(`squawk_monitor { interval = "bad" }`),
			wantErr: "squawk_monitor.interval",
		},
		{
			name: "poll interval too short",
			config: `
location {
  lat       = 0.0
  lon       = 0.0
  radius_km = 50.0
}
opensky {
  id     = "test"
  secret = "test"
}
poll_interval = "5s"
redis { addr = "localhost:6379" }
postgres { dsn = "postgres://localhost/test" }
`,
			wantErr: "poll_interval must be at least 10s",
		},
		{
			name:    "invalid enrichment refresh",
			config:  validBase(`enrichment_refresh = "bad"`),
			wantErr: "enrichment_refresh",
		},
		{
			name: "invalid retention alerts max age",
			config: validBase(`retention {
  sightings_max_age = "720h"
  alerts_max_age    = "bad"
}`),
			wantErr: "retention.alerts_max_age",
		},
		{
			name: "invalid retention routes max age",
			config: validBase(`retention {
  sightings_max_age = "720h"
  alerts_max_age    = "168h"
  routes_max_age    = "bad"
}`),
			wantErr: "retention.routes_max_age",
		},
		{
			name: "invalid retention interval",
			config: validBase(`retention {
  sightings_max_age = "720h"
  alerts_max_age    = "168h"
  interval          = "bad"
}`),
			wantErr: "retention.interval",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTemp(t, tt.config)
			_, err := Load(path)
			if err == nil {
				t.Fatal("Load() expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// TestLoad_PerSourcePollInterval verifies that opensky.poll_interval and
// dump1090.poll_interval are parsed and surface on the typed config so
// main.go can pick them over the top-level fallback.
func TestLoad_PerSourcePollInterval(t *testing.T) {
	content := `
location {
  lat       = 34.0928
  lon       = -118.3287
  radius_km = 50.0
}

opensky {
  id            = "test-client"
  secret        = "test-secret"
  poll_interval = "120s"
}

dump1090 {
  url           = "http://piaware.local/skyaware"
  poll_interval = "5s"
}

poll_interval = "30s"

redis {
  addr = "localhost:6379"
}

postgres {
  dsn = "postgres://user:pass@localhost:5432/testdb?sslmode=disable"
}
`
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.OpenSky.Interval != 120*time.Second {
		t.Errorf("OpenSky.Interval = %v, want 120s", cfg.OpenSky.Interval)
	}
	if cfg.Dump1090 == nil {
		t.Fatal("Dump1090 should be set")
	}
	if cfg.Dump1090.Interval != 5*time.Second {
		t.Errorf("Dump1090.Interval = %v, want 5s", cfg.Dump1090.Interval)
	}
	if cfg.Dump1090.URL != "http://piaware.local/skyaware" {
		t.Errorf("Dump1090.URL = %q", cfg.Dump1090.URL)
	}
	if cfg.Poll != 30*time.Second {
		t.Errorf("Poll = %v, want 30s", cfg.Poll)
	}
}

// TestLoad_PerSourcePollInterval_ZeroWhenUnset verifies that omitted
// per-source poll_intervals leave the typed Interval field as zero so
// callers know to fall back to the top-level default.
func TestLoad_PerSourcePollInterval_ZeroWhenUnset(t *testing.T) {
	content := `
location {
  lat       = 34.0928
  lon       = -118.3287
  radius_km = 50.0
}

opensky {
  id     = "test-client"
  secret = "test-secret"
}

dump1090 {
  url = "http://piaware.local/skyaware"
}

poll_interval = "30s"

redis {
  addr = "localhost:6379"
}

postgres {
  dsn = "postgres://user:pass@localhost:5432/testdb?sslmode=disable"
}
`
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.OpenSky.Interval != 0 {
		t.Errorf("OpenSky.Interval = %v, want 0", cfg.OpenSky.Interval)
	}
	if cfg.Dump1090.Interval != 0 {
		t.Errorf("Dump1090.Interval = %v, want 0", cfg.Dump1090.Interval)
	}
}

// TestLoad_PerSourcePollInterval_Validation rejects invalid durations and
// enforces each source's minimum cadence.
func TestLoad_PerSourcePollInterval_Validation(t *testing.T) {
	base := `
location {
  lat       = 34.0928
  lon       = -118.3287
  radius_km = 50.0
}

redis {
  addr = "localhost:6379"
}

postgres {
  dsn = "postgres://user:pass@localhost:5432/testdb?sslmode=disable"
}

poll_interval = "30s"
`
	tests := []struct {
		name    string
		extra   string
		wantErr string
	}{
		{
			name: "opensky interval below 10s floor",
			extra: `
opensky {
  id            = "x"
  secret        = "y"
  poll_interval = "2s"
}
`,
			wantErr: "opensky.poll_interval must be at least 10s",
		},
		{
			name: "opensky interval malformed",
			extra: `
opensky {
  id            = "x"
  secret        = "y"
  poll_interval = "notaduration"
}
`,
			wantErr: "opensky.poll_interval",
		},
		{
			name: "dump1090 interval below 1s floor",
			extra: `
opensky {
  id     = "x"
  secret = "y"
}
dump1090 {
  url           = "http://pi/skyaware"
  poll_interval = "100ms"
}
`,
			wantErr: "dump1090.poll_interval must be at least 1s",
		},
		{
			name: "dump1090 interval malformed",
			extra: `
opensky {
  id     = "x"
  secret = "y"
}
dump1090 {
  url           = "http://pi/skyaware"
  poll_interval = "whenever"
}
`,
			wantErr: "dump1090.poll_interval",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTemp(t, base+tt.extra)
			_, err := Load(path)
			if err == nil {
				t.Fatalf("Load() error = nil, want %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Load() error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// writeTemp creates a temporary HCL file and returns its path.
func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.hcl")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return path
}
