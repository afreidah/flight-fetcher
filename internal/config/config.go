// -------------------------------------------------------------------------------
// Config - Application Configuration
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Defines the configuration structure and HCL loading for the flight fetcher
// service. Covers location, poll interval, OpenSky credentials, and database
// connection settings. Secrets are templated into the HCL file by Vault.
// -------------------------------------------------------------------------------

package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/hcl/v2/hclsimple"
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Config holds all application configuration loaded from an HCL file.
type Config struct {
	PollInterval       string `hcl:"poll_interval"`
	EnrichmentRefresh  string `hcl:"enrichment_refresh,optional"`

	Location      Location             `hcl:"location,block"`
	OpenSky       OpenSkyConfig        `hcl:"opensky,block"`
	Redis         RedisConfig          `hcl:"redis,block"`
	Postgres      PostgresConfig       `hcl:"postgres,block"`
	AirLabs       *AirLabsConfig       `hcl:"airlabs,block"`
	FlightAware   *FlightAwareConfig   `hcl:"flightaware,block"`
	Server        *ServerConfig        `hcl:"server,block"`
	SquawkMonitor *SquawkMonitorConfig `hcl:"squawk_monitor,block"`
	Retention     *RetentionConfig     `hcl:"retention,block"`

	// Parsed durations populated by validate()
	Poll           time.Duration
	EnrichInterval time.Duration
}

// Location defines the center point and radius for aircraft search.
type Location struct {
	Lat      float64 `hcl:"lat"`
	Lon      float64 `hcl:"lon"`
	RadiusKm float64 `hcl:"radius_km"`
}

// OpenSkyConfig holds credentials for the OpenSky Network API.
type OpenSkyConfig struct {
	ID     string `hcl:"id"`
	Secret string `hcl:"secret"`
}

// RedisConfig holds connection parameters for Redis.
type RedisConfig struct {
	Addr     string `hcl:"addr"`
	Password string `hcl:"password,optional"`
	DB       int    `hcl:"db,optional"`
}

// PostgresConfig holds connection parameters for PostgreSQL.
type PostgresConfig struct {
	DSN string `hcl:"dsn"`
}

// ServerConfig holds settings for the optional web dashboard HTTP server.
type ServerConfig struct {
	Listen  string `hcl:"listen,optional"`
	Refresh int    `hcl:"refresh,optional"`
}

// RefreshSeconds returns the dashboard refresh interval in seconds.
// Defaults to 5 if not set or zero.
func (c *ServerConfig) RefreshSeconds() int {
	if c.Refresh <= 0 {
		return 5
	}
	return c.Refresh
}

// AirLabsConfig holds credentials for the AirLabs flight data API.
type AirLabsConfig struct {
	APIKey string `hcl:"api_key"`
}

// FlightAwareConfig holds credentials for the FlightAware AeroAPI.
type FlightAwareConfig struct {
	APIKey string `hcl:"api_key"`
}

// SquawkMonitorConfig holds settings for the global emergency squawk monitor.
type SquawkMonitorConfig struct {
	Interval string        `hcl:"interval"`
	Poll     time.Duration
}

// RetentionConfig holds settings for automatic data cleanup.
type RetentionConfig struct {
	SightingsMaxAge string `hcl:"sightings_max_age"`
	AlertsMaxAge    string `hcl:"alerts_max_age"`
	RoutesMaxAge    string `hcl:"routes_max_age,optional"`
	Interval        string `hcl:"interval,optional"`

	Sightings     time.Duration
	Alerts        time.Duration
	Routes        time.Duration
	CleanInterval time.Duration
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// Load reads and decodes an HCL configuration file at the given path,
// then validates the configuration values.
func Load(path string) (*Config, error) {
	var cfg Config
	if err := hclsimple.DecodeFile(path, nil, &cfg); err != nil {
		return nil, err
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &cfg, nil
}

// -------------------------------------------------------------------------
// INTERNALS
// -------------------------------------------------------------------------

// validate checks that all required fields are present, values are sane,
// and parses duration strings into the typed duration fields.
func (c *Config) validate() error {
	if c.Location.Lat < -90 || c.Location.Lat > 90 {
		return fmt.Errorf("location.lat must be between -90 and 90, got %f", c.Location.Lat)
	}
	if c.Location.Lon < -180 || c.Location.Lon > 180 {
		return fmt.Errorf("location.lon must be between -180 and 180, got %f", c.Location.Lon)
	}
	if c.Location.RadiusKm <= 0 {
		return errors.New("location.radius_km must be positive")
	}

	var err error
	c.Poll, err = time.ParseDuration(c.PollInterval)
	if err != nil {
		return fmt.Errorf("poll_interval: %w", err)
	}
	if c.Poll < 10*time.Second {
		return fmt.Errorf("poll_interval must be at least 10s, got %s", c.Poll)
	}
	if c.EnrichmentRefresh != "" {
		c.EnrichInterval, err = time.ParseDuration(c.EnrichmentRefresh)
		if err != nil {
			return fmt.Errorf("enrichment_refresh: %w", err)
		}
	} else {
		c.EnrichInterval = time.Hour
	}

	if c.OpenSky.ID == "" || c.OpenSky.Secret == "" {
		return errors.New("opensky.id and opensky.secret are required")
	}
	if c.Redis.Addr == "" {
		return errors.New("redis.addr is required")
	}
	if c.Postgres.DSN == "" {
		return errors.New("postgres.dsn is required")
	}
	if c.AirLabs != nil && c.AirLabs.APIKey == "" {
		return errors.New("airlabs.api_key is required when airlabs block is present")
	}
	if c.FlightAware != nil && c.FlightAware.APIKey == "" {
		return errors.New("flightaware.api_key is required when flightaware block is present")
	}
	if c.SquawkMonitor != nil {
		c.SquawkMonitor.Poll, err = time.ParseDuration(c.SquawkMonitor.Interval)
		if err != nil {
			return fmt.Errorf("squawk_monitor.interval: %w", err)
		}
	}
	if c.Retention != nil {
		r := c.Retention
		r.Sightings, err = time.ParseDuration(r.SightingsMaxAge)
		if err != nil {
			return fmt.Errorf("retention.sightings_max_age: %w", err)
		}
		r.Alerts, err = time.ParseDuration(r.AlertsMaxAge)
		if err != nil {
			return fmt.Errorf("retention.alerts_max_age: %w", err)
		}
		if r.RoutesMaxAge != "" {
			r.Routes, err = time.ParseDuration(r.RoutesMaxAge)
			if err != nil {
				return fmt.Errorf("retention.routes_max_age: %w", err)
			}
		} else {
			r.Routes = 24 * time.Hour
		}
		if r.Interval != "" {
			r.CleanInterval, err = time.ParseDuration(r.Interval)
			if err != nil {
				return fmt.Errorf("retention.interval: %w", err)
			}
		} else {
			r.CleanInterval = time.Hour
		}
	}
	return nil
}
