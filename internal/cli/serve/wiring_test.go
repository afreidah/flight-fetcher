// -------------------------------------------------------------------------------
// Flight Fetcher - Wiring Decision Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Covers the entrypoint's planning helpers: which flight sources are polled and
// at what interval, how the Redis TTL is derived from those intervals, and
// which route providers and notification backends a given config enables. The
// helpers are pure translations of config into plans, so these tests assert on
// the returned plan without standing up Redis, Postgres, or a network.
// -------------------------------------------------------------------------------

package serve

import (
	"slices"
	"testing"
	"time"

	"github.com/afreidah/flight-fetcher/internal/apiclient/hexdb"
	"github.com/afreidah/flight-fetcher/internal/config"
)

// -------------------------------------------------------------------------
// INTERVAL RESOLUTION
// -------------------------------------------------------------------------

// TestFirstNonZeroInterval verifies the per-source override falls back to the
// top-level default, and that non-positive values never win.
func TestFirstNonZeroInterval(t *testing.T) {
	tests := []struct {
		name      string
		intervals []time.Duration
		want      time.Duration
	}{
		{"no intervals", nil, 0},
		{"all zero", []time.Duration{0, 0}, 0},
		{"first wins", []time.Duration{10 * time.Second, 30 * time.Second}, 10 * time.Second},
		{"falls back to second", []time.Duration{0, 30 * time.Second}, 30 * time.Second},
		{"negative is skipped", []time.Duration{-5 * time.Second, 30 * time.Second}, 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstNonZeroInterval(tt.intervals...); got != tt.want {
				t.Errorf("firstNonZeroInterval(%v) = %v, want %v", tt.intervals, got, tt.want)
			}
		})
	}
}

// -------------------------------------------------------------------------
// FLIGHT SOURCES
// -------------------------------------------------------------------------

// TestPlannedSources verifies that OpenSky is always planned, that the antenna
// is appended only when a dump1090 block is present, and that each source
// resolves its own interval before falling back to the top-level default.
func TestPlannedSources(t *testing.T) {
	tests := []struct {
		name          string
		cfg           *config.Config
		wantNames     []string
		wantIntervals []time.Duration
	}{
		{
			name: "opensky only, top-level interval",
			cfg: &config.Config{
				Poll:    20 * time.Second,
				OpenSky: &config.OpenSkyConfig{},
			},
			wantNames:     []string{"opensky"},
			wantIntervals: []time.Duration{20 * time.Second},
		},
		{
			name: "opensky per-source interval overrides top level",
			cfg: &config.Config{
				Poll:    20 * time.Second,
				OpenSky: &config.OpenSkyConfig{Interval: 45 * time.Second},
			},
			wantNames:     []string{"opensky"},
			wantIntervals: []time.Duration{45 * time.Second},
		},
		{
			name: "antenna appended and inherits top-level interval",
			cfg: &config.Config{
				Poll:     20 * time.Second,
				OpenSky:  &config.OpenSkyConfig{},
				Dump1090: &config.Dump1090Config{URL: "http://antenna.local"},
			},
			wantNames:     []string{"opensky", "antenna"},
			wantIntervals: []time.Duration{20 * time.Second, 20 * time.Second},
		},
		{
			name: "antenna polls faster than opensky",
			cfg: &config.Config{
				Poll:     20 * time.Second,
				OpenSky:  &config.OpenSkyConfig{},
				Dump1090: &config.Dump1090Config{URL: "http://antenna.local", Interval: 5 * time.Second},
			},
			wantNames:     []string{"opensky", "antenna"},
			wantIntervals: []time.Duration{20 * time.Second, 5 * time.Second},
		},
		{
			name: "antenna only when the opensky block is absent",
			cfg: &config.Config{
				Poll:     20 * time.Second,
				Dump1090: &config.Dump1090Config{URL: "http://antenna.local"},
			},
			wantNames:     []string{"antenna"},
			wantIntervals: []time.Duration{20 * time.Second},
		},
		{
			name: "antenna only with its own interval",
			cfg: &config.Config{
				Poll:     20 * time.Second,
				Dump1090: &config.Dump1090Config{URL: "http://antenna.local", Interval: 5 * time.Second},
			},
			wantNames:     []string{"antenna"},
			wantIntervals: []time.Duration{5 * time.Second},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specs := plannedSources(tt.cfg)

			names := make([]string, 0, len(specs))
			intervals := make([]time.Duration, 0, len(specs))
			for i, spec := range specs {
				names = append(names, spec.name)
				intervals = append(intervals, spec.interval)
				if spec.source == nil {
					t.Errorf("spec[%d].source is nil", i)
				}
			}

			if !slices.Equal(names, tt.wantNames) {
				t.Errorf("plannedSources() names = %v, want %v", names, tt.wantNames)
			}
			if !slices.Equal(intervals, tt.wantIntervals) {
				t.Errorf("plannedSources() intervals = %v, want %v", intervals, tt.wantIntervals)
			}
		})
	}
}

// TestSourceNames verifies the dashboard gets the planned names in order.
func TestSourceNames(t *testing.T) {
	tests := []struct {
		name  string
		specs []sourceSpec
		want  []string
	}{
		{"no specs", nil, []string{}},
		{"single", []sourceSpec{{name: "opensky"}}, []string{"opensky"}},
		{
			"order preserved",
			[]sourceSpec{{name: "opensky"}, {name: "antenna"}},
			[]string{"opensky", "antenna"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sourceNames(tt.specs); !slices.Equal(got, tt.want) {
				t.Errorf("sourceNames() = %v, want %v", got, tt.want)
			}
		})
	}
}

// -------------------------------------------------------------------------
// CACHE TTL
// -------------------------------------------------------------------------

// TestCacheTTL verifies the TTL covers three cycles of the slowest poller, so
// a fast antenna cannot shorten the window an aircraft heard only by OpenSky
// survives in the cache.
func TestCacheTTL(t *testing.T) {
	tests := []struct {
		name        string
		specs       []sourceSpec
		wantSlowest time.Duration
		wantTTL     time.Duration
	}{
		{"no specs", nil, 0, 0},
		{
			"single source",
			[]sourceSpec{{interval: 20 * time.Second}},
			20 * time.Second,
			60 * time.Second,
		},
		{
			"slowest wins regardless of order",
			[]sourceSpec{{interval: 5 * time.Second}, {interval: 30 * time.Second}},
			30 * time.Second,
			90 * time.Second,
		},
		{
			"slowest listed first",
			[]sourceSpec{{interval: 30 * time.Second}, {interval: 5 * time.Second}},
			30 * time.Second,
			90 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slowest, ttl := cacheTTL(tt.specs)
			if slowest != tt.wantSlowest {
				t.Errorf("cacheTTL() slowest = %v, want %v", slowest, tt.wantSlowest)
			}
			if ttl != tt.wantTTL {
				t.Errorf("cacheTTL() ttl = %v, want %v", ttl, tt.wantTTL)
			}
		})
	}
}

// -------------------------------------------------------------------------
// ENRICHMENT SOURCES
// -------------------------------------------------------------------------

// TestPlannedAircraftSources verifies the unauthenticated HexDB lookup is tried
// before the credit-spending OpenSky one, and that HexDB alone is a valid chain
// when no credentials are configured.
func TestPlannedAircraftSources(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		want []string
	}{
		{
			"hexdb first, then opensky",
			&config.Config{OpenSky: &config.OpenSkyConfig{}},
			[]string{"hexdb", "opensky"},
		},
		{
			"hexdb alone when the opensky block is absent",
			&config.Config{},
			[]string{"hexdb"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sources := plannedAircraftSources(tt.cfg, hexdb.NewClient())
			if len(sources) != len(tt.want) {
				t.Fatalf("plannedAircraftSources() returned %d sources, want %d", len(sources), len(tt.want))
			}
			for i, s := range sources {
				if s.Name != tt.want[i] {
					t.Errorf("source[%d].Name = %q, want %q", i, s.Name, tt.want[i])
				}
				if s.Fn == nil {
					t.Errorf("source[%d].Fn is nil", i)
				}
			}
		})
	}
}

// TestPlannedRouteSources verifies both providers are optional, that a
// configured block with an empty key is treated as absent, and that AirLabs is
// tried before FlightAware when both are enabled.
func TestPlannedRouteSources(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		want []string
	}{
		{"neither configured", &config.Config{}, nil},
		{
			"airlabs block with empty key",
			&config.Config{AirLabs: &config.AirLabsConfig{}},
			nil,
		},
		{
			"flightaware block with empty key",
			&config.Config{FlightAware: &config.FlightAwareConfig{}},
			nil,
		},
		{
			"airlabs only",
			&config.Config{AirLabs: &config.AirLabsConfig{APIKey: "al-key"}},
			[]string{"airlabs"},
		},
		{
			"flightaware only",
			&config.Config{FlightAware: &config.FlightAwareConfig{APIKey: "fa-key"}},
			[]string{"flightaware"},
		},
		{
			"both, airlabs first",
			&config.Config{
				AirLabs:     &config.AirLabsConfig{APIKey: "al-key"},
				FlightAware: &config.FlightAwareConfig{APIKey: "fa-key"},
			},
			[]string{"airlabs", "flightaware"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sources := plannedRouteSources(tt.cfg)
			if len(sources) != len(tt.want) {
				t.Fatalf("plannedRouteSources() returned %d sources, want %d", len(sources), len(tt.want))
			}
			for i, s := range sources {
				if s.Name != tt.want[i] {
					t.Errorf("source[%d].Name = %q, want %q", i, s.Name, tt.want[i])
				}
				if s.Fn == nil {
					t.Errorf("source[%d].Fn is nil", i)
				}
			}
		})
	}
}

// -------------------------------------------------------------------------
// NOTIFIERS
// -------------------------------------------------------------------------

// TestPlannedNotifiers verifies each configured block becomes one registration,
// that multiple targets of the same kind are all registered, and that an
// absent notifications block leaves the manager a no-op.
func TestPlannedNotifiers(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		want []string
	}{
		{"no notifications block", &config.Config{}, nil},
		{
			"empty notifications block",
			&config.Config{Notifications: &config.NotificationsConfig{}},
			nil,
		},
		{
			"discord only",
			&config.Config{Notifications: &config.NotificationsConfig{
				Discord: []config.DiscordConfig{{WebhookURL: "https://discord.example/hook"}},
			}},
			[]string{"discord"},
		},
		{
			"telegram only",
			&config.Config{Notifications: &config.NotificationsConfig{
				Telegram: []config.TelegramConfig{{BotToken: "token", ChatID: "chat"}},
			}},
			[]string{"telegram"},
		},
		{
			"multiple targets of both kinds",
			&config.Config{Notifications: &config.NotificationsConfig{
				Discord: []config.DiscordConfig{
					{WebhookURL: "https://discord.example/one"},
					{WebhookURL: "https://discord.example/two"},
				},
				Telegram: []config.TelegramConfig{{BotToken: "token", ChatID: "chat"}},
			}},
			[]string{"discord", "discord", "telegram"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specs := plannedNotifiers(tt.cfg)
			if len(specs) != len(tt.want) {
				t.Fatalf("plannedNotifiers() returned %d specs, want %d", len(specs), len(tt.want))
			}
			for i, spec := range specs {
				if spec.name != tt.want[i] {
					t.Errorf("spec[%d].name = %q, want %q", i, spec.name, tt.want[i])
				}
				if spec.notifier == nil {
					t.Errorf("spec[%d].notifier is nil", i)
				}
			}
		})
	}
}
