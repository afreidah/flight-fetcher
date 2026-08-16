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

// rawConfig mirrors the HCL file structure with string durations. The
// untagged poll and enrich fields hold the parsed forms of PollInterval and
// EnrichmentRefresh; they are populated by parseIntervals during validation
// and are invisible to the HCL decoder, which only considers tagged fields.
type rawConfig struct {
	PollInterval      string `hcl:"poll_interval"`
	EnrichmentRefresh string `hcl:"enrichment_refresh,optional"`

	poll   time.Duration
	enrich time.Duration

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

// OpenSkyConfig holds credentials for the OpenSky Network API and the
// optional per-source poll interval. When Interval is zero, the top-level
// poll_interval is used as the fallback.
type OpenSkyConfig struct {
	ID           string `hcl:"id"`
	Secret       string `hcl:"secret" json:"-"`
	PollInterval string `hcl:"poll_interval,optional"`

	// Interval is populated during Load from PollInterval; zero means unset.
	Interval time.Duration
}

// RedisConfig holds connection parameters for Redis.
type RedisConfig struct {
	Addr     string `hcl:"addr"`
	Password string `hcl:"password,optional" json:"-"`
	DB       int    `hcl:"db,optional"`
}

// PostgresConfig holds connection parameters for PostgreSQL. The DSN embeds
// the database password, so it carries the same json:"-" guard as the other
// credential-bearing fields.
type PostgresConfig struct {
	DSN string `hcl:"dsn" json:"-"`
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
	APIKey string `hcl:"api_key" json:"-"`
}

// FlightAwareConfig holds credentials for the FlightAware AeroAPI.
type FlightAwareConfig struct {
	APIKey string `hcl:"api_key" json:"-"`
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
// Possession of the webhook URL is sufficient to post to the channel, so it
// is treated as a credential.
type DiscordConfig struct {
	WebhookURL string `hcl:"webhook_url" json:"-"`
}

// TelegramConfig holds settings for a Telegram Bot API notification target.
type TelegramConfig struct {
	BotToken string `hcl:"bot_token" json:"-"`
	ChatID   string `hcl:"chat_id"`
}

// Dump1090Config holds settings for a local dump1090/readsb ADS-B receiver
// and the optional per-source poll interval. When Interval is zero, the
// top-level poll_interval is used as the fallback.
type Dump1090Config struct {
	URL          string `hcl:"url"`
	PollInterval string `hcl:"poll_interval,optional"`

	// Interval is populated during Load from PollInterval; zero means unset.
	Interval time.Duration
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

// parse validates the raw HCL input and produces a Config with parsed
// durations. Validation runs one step per config block, in the slice order
// below, and stops at the first failure, so the error a user sees names the
// earliest offending block rather than an arbitrary one.
func (r *rawConfig) parse() (*Config, error) {
	for _, validate := range []func() error{
		r.validateLocation,
		r.parseIntervals,
		r.parseOpenSky,
		r.validateRedis,
		r.validatePostgres,
		r.validateAirLabs,
		r.validateFlightAware,
		r.parseDump1090,
	} {
		if err := validate(); err != nil {
			return nil, err
		}
	}

	cfg := &Config{
		Poll:           r.poll,
		EnrichInterval: r.enrich,
		Location:       r.Location,
		OpenSky:        r.OpenSky,
		Redis:          r.Redis,
		Postgres:       r.Postgres,
		AirLabs:        r.AirLabs,
		FlightAware:    r.FlightAware,
		Server:         r.Server,
		Dump1090:       r.Dump1090,
	}
	if err := r.parseOptionalBlocks(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// validateLocation checks that the receiver coordinates are on the globe and
// the search radius is usable.
func (r *rawConfig) validateLocation() error {
	if r.Location.Lat < -90 || r.Location.Lat > 90 {
		return fmt.Errorf("location.lat must be between -90 and 90, got %f", r.Location.Lat)
	}
	if r.Location.Lon < -180 || r.Location.Lon > 180 {
		return fmt.Errorf("location.lon must be between -180 and 180, got %f", r.Location.Lon)
	}
	if r.Location.RadiusKm <= 0 {
		return errors.New("location.radius_km must be positive")
	}
	return nil
}

// parseIntervals parses the two top-level durations into r.poll and r.enrich.
// The poll floor of 10s keeps the default source inside the OpenSky credit
// budget. Enrichment refresh defaults to an hour when omitted.
func (r *rawConfig) parseIntervals() error {
	poll, err := time.ParseDuration(r.PollInterval)
	if err != nil {
		return fmt.Errorf("poll_interval: %w", err)
	}
	if poll < 10*time.Second {
		return fmt.Errorf("poll_interval must be at least 10s, got %s", poll)
	}
	r.poll = poll

	r.enrich = time.Hour
	if r.EnrichmentRefresh != "" {
		enrich, err := time.ParseDuration(r.EnrichmentRefresh)
		if err != nil {
			return fmt.Errorf("enrichment_refresh: %w", err)
		}
		r.enrich = enrich
	}
	return nil
}

// parseOpenSky checks that credentials are present and parses the optional
// per-source poll interval, which carries the same 10s floor as the top-level
// default because it overrides it for the same credit-metered API.
func (r *rawConfig) parseOpenSky() error {
	if r.OpenSky.ID == "" || r.OpenSky.Secret == "" {
		return errors.New("opensky.id and opensky.secret are required")
	}
	if r.OpenSky.PollInterval == "" {
		return nil
	}
	d, err := time.ParseDuration(r.OpenSky.PollInterval)
	if err != nil {
		return fmt.Errorf("opensky.poll_interval: %w", err)
	}
	if d < 10*time.Second {
		return fmt.Errorf("opensky.poll_interval must be at least 10s, got %s", d)
	}
	r.OpenSky.Interval = d
	return nil
}

// validateRedis checks that a Redis address is configured.
func (r *rawConfig) validateRedis() error {
	if r.Redis.Addr == "" {
		return errors.New("redis.addr is required")
	}
	return nil
}

// validatePostgres checks that a Postgres DSN is configured.
func (r *rawConfig) validatePostgres() error {
	if r.Postgres.DSN == "" {
		return errors.New("postgres.dsn is required")
	}
	return nil
}

// validateAirLabs checks that the optional AirLabs block carries a key when
// present, since a keyless block would silently disable route enrichment.
func (r *rawConfig) validateAirLabs() error {
	if r.AirLabs != nil && r.AirLabs.APIKey == "" {
		return errors.New("airlabs.api_key is required when airlabs block is present")
	}
	return nil
}

// validateFlightAware checks that the optional FlightAware block carries a key
// when present.
func (r *rawConfig) validateFlightAware() error {
	if r.FlightAware != nil && r.FlightAware.APIKey == "" {
		return errors.New("flightaware.api_key is required when flightaware block is present")
	}
	return nil
}

// parseDump1090 validates the optional local receiver block. Its poll floor is
// 1s rather than 10s: the receiver is on the local network with no credit
// budget, so it is polled far more often than the wide-area source.
func (r *rawConfig) parseDump1090() error {
	if r.Dump1090 == nil {
		return nil
	}
	if r.Dump1090.URL == "" {
		return errors.New("dump1090.url is required when dump1090 block is present")
	}
	if r.Dump1090.PollInterval == "" {
		return nil
	}
	d, err := time.ParseDuration(r.Dump1090.PollInterval)
	if err != nil {
		return fmt.Errorf("dump1090.poll_interval: %w", err)
	}
	if d < time.Second {
		return fmt.Errorf("dump1090.poll_interval must be at least 1s, got %s", d)
	}
	r.Dump1090.Interval = d
	return nil
}

// parseOptionalBlocks fills in the Config fields whose blocks are absent by
// default. Each stays nil when its block is omitted, which is how the
// composition root decides whether to start the corresponding component.
func (r *rawConfig) parseOptionalBlocks(cfg *Config) error {
	if r.SquawkMonitor != nil {
		smPoll, err := time.ParseDuration(r.SquawkMonitor.Interval)
		if err != nil {
			return fmt.Errorf("squawk_monitor.interval: %w", err)
		}
		cfg.SquawkMonitor = &SquawkMonitorConfig{Poll: smPoll}
	}

	if r.Retention != nil {
		ret, err := parseRetention(r.Retention)
		if err != nil {
			return err
		}
		cfg.Retention = ret
	}

	if r.Notifications != nil {
		notif, err := parseNotifications(r.Notifications)
		if err != nil {
			return err
		}
		cfg.Notifications = notif
	}
	return nil
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
