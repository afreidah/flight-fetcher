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
	if cfg.PollInterval != "20s" {
		t.Errorf("PollInterval = %q, want %q", cfg.PollInterval, "20s")
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
	sightings, alerts, _, interval, err := cfg.Retention.RetentionDurations()
	if err != nil {
		t.Fatalf("RetentionDurations() error = %v", err)
	}
	if sightings != 720*time.Hour {
		t.Errorf("sightings = %v, want 720h", sightings)
	}
	if alerts != 168*time.Hour {
		t.Errorf("alerts = %v, want 168h", alerts)
	}
	if interval != 2*time.Hour {
		t.Errorf("interval = %v, want 2h", interval)
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
	_, _, _, interval, err := cfg.Retention.RetentionDurations()
	if err != nil {
		t.Fatalf("RetentionDurations() error = %v", err)
	}
	if interval != time.Hour {
		t.Errorf("interval = %v, want 1h (default)", interval)
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
	d, err := cfg.SquawkMonitor.SquawkMonitorDuration()
	if err != nil {
		t.Fatalf("SquawkMonitorDuration() error = %v", err)
	}
	if d != 60*time.Second {
		t.Errorf("SquawkMonitorDuration() = %v, want 60s", d)
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

// TestPollDuration_Valid verifies that a valid duration string is parsed correctly.
func TestPollDuration_Valid(t *testing.T) {
	cfg := &Config{PollInterval: "30s"}
	d, err := cfg.PollDuration()
	if err != nil {
		t.Fatalf("PollDuration() error = %v", err)
	}
	if d.Seconds() != 30 {
		t.Errorf("PollDuration() = %v, want 30s", d)
	}
}

// TestPollDuration_Invalid verifies that an invalid duration string returns an error.
func TestPollDuration_Invalid(t *testing.T) {
	cfg := &Config{PollInterval: "notaduration"}
	_, err := cfg.PollDuration()
	if err == nil {
		t.Error("PollDuration() expected error for invalid duration, got nil")
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
			name:    "invalid squawk monitor interval",
			config:  validBase(`squawk_monitor { interval = "bad" }`),
			wantErr: "squawk_monitor.interval",
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
