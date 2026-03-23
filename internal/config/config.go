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
	"time"

	"github.com/hashicorp/hcl/v2/hclsimple"
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Config holds all application configuration loaded from an HCL file.
type Config struct {
	Location     Location       `hcl:"location,block"`
	OpenSky      OpenSkyConfig  `hcl:"opensky,block"`
	PollInterval string         `hcl:"poll_interval"`
	Redis        RedisConfig    `hcl:"redis,block"`
	Postgres     PostgresConfig `hcl:"postgres,block"`
	Server       *ServerConfig       `hcl:"server,block"`
	AirLabs      *AirLabsConfig     `hcl:"airlabs,block"`
	SquawkMonitor *SquawkMonitorConfig `hcl:"squawk_monitor,block"`
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
	Listen string `hcl:"listen,optional"`
}

// AirLabsConfig holds credentials for the AirLabs flight data API.
type AirLabsConfig struct {
	APIKey string `hcl:"api_key"`
}

// SquawkMonitorConfig holds settings for the global emergency squawk monitor.
type SquawkMonitorConfig struct {
	Interval string `hcl:"interval"`
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// PollDuration parses the PollInterval string into a time.Duration.
func (c *Config) PollDuration() (time.Duration, error) {
	return time.ParseDuration(c.PollInterval)
}

// SquawkMonitorDuration parses the squawk monitor interval into a time.Duration.
func (c *SquawkMonitorConfig) SquawkMonitorDuration() (time.Duration, error) {
	return time.ParseDuration(c.Interval)
}

// Load reads and decodes an HCL configuration file at the given path.
func Load(path string) (*Config, error) {
	var cfg Config
	if err := hclsimple.DecodeFile(path, nil, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
