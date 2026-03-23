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
	"testing"
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
