// -------------------------------------------------------------------------------
// Config - Application Configuration
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Defines the configuration structure and HCL loading for the flight fetcher
// service. Covers location, poll interval, OpenSky credentials, database
// connection settings, and notification backends. Secrets are templated into
// the HCL file by Vault.
// -------------------------------------------------------------------------------

package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/hcl/v2/hclsimple"
)

// -------------------------------------------------------------------------
// RAW HCL TYPES (unexported, used only for deserialization)
// -------------------------------------------------------------------------

// rawConfig mirrors the HCL file structure with string durations.
type rawConfig struct {
	PollInterval      string `hcl:"poll_interval"`
	EnrichmentRefresh string `hcl:"enrichment_refresh,optional"`

	Location      Location                `hcl:"location,block"`
	OpenSky       OpenSkyConfig           `hcl:"opensky,block"`
	Redis         RedisConfig             `hcl:"redis,block"`
	Postgres      PostgresConfig          `hcl:"postgres,block"`
	AirLabs       *AirLabsConfig          `hcl:"airlabs,block"`
	FlightAware   *FlightAwareConfig      `hcl:"flightaware,block"`
	Server        *ServerConfig           `hcl:"server,block"`
	SquawkMonitor *rawSquawkMonitorConfig `hcl:"squawk_monitor,block"`
	Retention     *rawRetentionConfig     `hcl:"retention,block"`
	Notifications *rawNotificationsConfig `hcl:"notifications,block"`
	Dump1090      *Dump1090Config         `hcl:"dump1090,block"`
}

type rawSquawkMonitorConfig struct {
	Interval string `hcl:"interval"`
}

type rawRetentionConfig struct {
	SightingsMaxAge string `hcl:"sightings_max_age"`
	AlertsMaxAge    string `hcl:"alerts_max_age"`
	RoutesMaxAge    string `hcl:"routes_max_age,optional"`
	Interval        string `hcl:"interval,optional"`
}

type rawNotificationsConfig struct {
	Discord  []DiscordConfig  `hcl:"discord,block"`
	Telegram []TelegramConfig `hcl:"telegram,block"`
}

// -------------------------------------------------------------------------
// PUBLIC TYPES
// -------------------------------------------------------------------------

// Config holds all validated application configuration with parsed durations.
type Config struct {
	Poll           time.Duration
	EnrichInterval time.Duration

	Location    Location
	OpenSky     OpenSkyConfig
	Redis       RedisConfig
	Postgres    PostgresConfig
	AirLabs     *AirLabsConfig
	FlightAware *FlightAwareConfig
	Server      *ServerConfig

	SquawkMonitor *SquawkMonitorConfig
	Retention     *RetentionConfig
	Notifications *NotificationsConfig
	Dump1090      *Dump1090Config
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

// SquawkMonitorConfig holds validated settings for the global emergency squawk monitor.
type SquawkMonitorConfig struct {
	Poll time.Duration
}

// RetentionConfig holds validated settings for automatic data cleanup.
type RetentionConfig struct {
	Sightings     time.Duration
	Alerts        time.Duration
	Routes        time.Duration
	CleanInterval time.Duration
}

// NotificationsConfig holds all configured notification backends.
type NotificationsConfig struct {
	Discord  []DiscordConfig
	Telegram []TelegramConfig
}

// DiscordConfig holds settings for a Discord webhook notification target.
type DiscordConfig struct {
	WebhookURL string `hcl:"webhook_url"`
}

// TelegramConfig holds settings for a Telegram Bot API notification target.
type TelegramConfig struct {
	BotToken string `hcl:"bot_token"`
	ChatID   string `hcl:"chat_id"`
}

// Dump1090Config holds settings for a local dump1090/readsb ADS-B receiver.
type Dump1090Config struct {
	URL string `hcl:"url"`
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// Load reads and decodes an HCL configuration file at the given path,
// validates configuration values, and returns a Config with parsed durations.
func Load(path string) (*Config, error) {
	var raw rawConfig
	if err := hclsimple.DecodeFile(path, nil, &raw); err != nil {
		return nil, err
	}
	return raw.parse()
}

// -------------------------------------------------------------------------
// INTERNALS
// -------------------------------------------------------------------------

// parse validates the raw HCL input and produces a Config with parsed durations.
func (r *rawConfig) parse() (*Config, error) {
	if r.Location.Lat < -90 || r.Location.Lat > 90 {
		return nil, fmt.Errorf("location.lat must be between -90 and 90, got %f", r.Location.Lat)
	}
	if r.Location.Lon < -180 || r.Location.Lon > 180 {
		return nil, fmt.Errorf("location.lon must be between -180 and 180, got %f", r.Location.Lon)
	}
	if r.Location.RadiusKm <= 0 {
		return nil, errors.New("location.radius_km must be positive")
	}

	poll, err := time.ParseDuration(r.PollInterval)
	if err != nil {
		return nil, fmt.Errorf("poll_interval: %w", err)
	}
	if poll < 10*time.Second {
		return nil, fmt.Errorf("poll_interval must be at least 10s, got %s", poll)
	}

	enrichInterval := time.Hour
	if r.EnrichmentRefresh != "" {
		enrichInterval, err = time.ParseDuration(r.EnrichmentRefresh)
		if err != nil {
			return nil, fmt.Errorf("enrichment_refresh: %w", err)
		}
	}

	if r.OpenSky.ID == "" || r.OpenSky.Secret == "" {
		return nil, errors.New("opensky.id and opensky.secret are required")
	}
	if r.Redis.Addr == "" {
		return nil, errors.New("redis.addr is required")
	}
	if r.Postgres.DSN == "" {
		return nil, errors.New("postgres.dsn is required")
	}
	if r.AirLabs != nil && r.AirLabs.APIKey == "" {
		return nil, errors.New("airlabs.api_key is required when airlabs block is present")
	}
	if r.FlightAware != nil && r.FlightAware.APIKey == "" {
		return nil, errors.New("flightaware.api_key is required when flightaware block is present")
	}
	if r.Dump1090 != nil && r.Dump1090.URL == "" {
		return nil, errors.New("dump1090.url is required when dump1090 block is present")
	}

	cfg := &Config{
		Poll:           poll,
		EnrichInterval: enrichInterval,
		Location:       r.Location,
		OpenSky:        r.OpenSky,
		Redis:          r.Redis,
		Postgres:       r.Postgres,
		AirLabs:        r.AirLabs,
		FlightAware:    r.FlightAware,
		Server:         r.Server,
		Dump1090:       r.Dump1090,
	}

	if r.SquawkMonitor != nil {
		smPoll, err := time.ParseDuration(r.SquawkMonitor.Interval)
		if err != nil {
			return nil, fmt.Errorf("squawk_monitor.interval: %w", err)
		}
		cfg.SquawkMonitor = &SquawkMonitorConfig{Poll: smPoll}
	}

	if r.Retention != nil {
		ret, err := parseRetention(r.Retention)
		if err != nil {
			return nil, err
		}
		cfg.Retention = ret
	}

	if r.Notifications != nil {
		notif, err := parseNotifications(r.Notifications)
		if err != nil {
			return nil, err
		}
		cfg.Notifications = notif
	}

	return cfg, nil
}

// parseRetention validates and parses the raw retention config.
func parseRetention(r *rawRetentionConfig) (*RetentionConfig, error) {
	sightings, err := time.ParseDuration(r.SightingsMaxAge)
	if err != nil {
		return nil, fmt.Errorf("retention.sightings_max_age: %w", err)
	}
	alerts, err := time.ParseDuration(r.AlertsMaxAge)
	if err != nil {
		return nil, fmt.Errorf("retention.alerts_max_age: %w", err)
	}

	routes := 24 * time.Hour
	if r.RoutesMaxAge != "" {
		routes, err = time.ParseDuration(r.RoutesMaxAge)
		if err != nil {
			return nil, fmt.Errorf("retention.routes_max_age: %w", err)
		}
	}

	cleanInterval := time.Hour
	if r.Interval != "" {
		cleanInterval, err = time.ParseDuration(r.Interval)
		if err != nil {
			return nil, fmt.Errorf("retention.interval: %w", err)
		}
	}

	return &RetentionConfig{
		Sightings:     sightings,
		Alerts:        alerts,
		Routes:        routes,
		CleanInterval: cleanInterval,
	}, nil
}

// parseNotifications validates each notification backend config.
func parseNotifications(r *rawNotificationsConfig) (*NotificationsConfig, error) {
	for i, d := range r.Discord {
		if d.WebhookURL == "" {
			return nil, fmt.Errorf("notifications.discord[%d].webhook_url is required", i)
		}
	}
	for i, t := range r.Telegram {
		if t.BotToken == "" {
			return nil, fmt.Errorf("notifications.telegram[%d].bot_token is required", i)
		}
		if t.ChatID == "" {
			return nil, fmt.Errorf("notifications.telegram[%d].chat_id is required", i)
		}
	}
	return &NotificationsConfig{
		Discord:  r.Discord,
		Telegram: r.Telegram,
	}, nil
}
