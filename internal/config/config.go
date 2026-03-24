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
	Interval string `hcl:"interval"`
}

// RetentionConfig holds settings for automatic data cleanup.
type RetentionConfig struct {
	SightingsMaxAge string `hcl:"sightings_max_age"`
	AlertsMaxAge    string `hcl:"alerts_max_age"`
	RoutesMaxAge    string `hcl:"routes_max_age,optional"`
	Interval        string `hcl:"interval,optional"`
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// PollDuration parses the PollInterval string into a time.Duration.
func (c *Config) PollDuration() (time.Duration, error) {
	return time.ParseDuration(c.PollInterval)
}

// EnrichmentRefreshDuration parses the enrichment refresh interval.
// Defaults to 1 hour if not set.
func (c *Config) EnrichmentRefreshDuration() (time.Duration, error) {
	if c.EnrichmentRefresh == "" {
		return time.Hour, nil
	}
	return time.ParseDuration(c.EnrichmentRefresh)
}

// SquawkMonitorDuration parses the squawk monitor interval into a time.Duration.
func (c *SquawkMonitorConfig) SquawkMonitorDuration() (time.Duration, error) {
	return time.ParseDuration(c.Interval)
}

// RetentionDurations parses the retention config into durations. Interval
// defaults to 1 hour and routes_max_age defaults to 24 hours if not specified.
func (c *RetentionConfig) RetentionDurations() (sightings, alerts, routes, interval time.Duration, err error) {
	sightings, err = time.ParseDuration(c.SightingsMaxAge)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	alerts, err = time.ParseDuration(c.AlertsMaxAge)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	if c.RoutesMaxAge != "" {
		routes, err = time.ParseDuration(c.RoutesMaxAge)
		if err != nil {
			return 0, 0, 0, 0, err
		}
	} else {
		routes = 24 * time.Hour
	}
	if c.Interval != "" {
		interval, err = time.ParseDuration(c.Interval)
		if err != nil {
			return 0, 0, 0, 0, err
		}
	} else {
		interval = time.Hour
	}
	return sightings, alerts, routes, interval, nil
}

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

// validate checks that all required fields are present and values are sane.
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
	if _, err := c.PollDuration(); err != nil {
		return fmt.Errorf("poll_interval: %w", err)
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
		if _, err := c.SquawkMonitor.SquawkMonitorDuration(); err != nil {
			return fmt.Errorf("squawk_monitor.interval: %w", err)
		}
	}
	if c.Retention != nil {
		if _, _, _, _, err := c.Retention.RetentionDurations(); err != nil {
			return fmt.Errorf("retention: %w", err)
		}
	}
	return nil
}
