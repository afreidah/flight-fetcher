// -------------------------------------------------------------------------------
// Flight Fetcher - Wiring Decisions
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// The decisions main makes about what to construct, separated from the
// construction itself. Each function here is a pure translation of validated
// config into a plan: which flight sources to poll and how often, which route
// providers to try, which notification backends to register, and how long
// cached flight state should live. Keeping them free of I/O is what makes the
// entrypoint's logic testable without standing up Redis, Postgres, or a
// network.
// -------------------------------------------------------------------------------

package serve

import (
	"context"
	"time"

	"github.com/afreidah/flight-fetcher/internal/aircraft"
	"github.com/afreidah/flight-fetcher/internal/apiclient/airlabs"
	"github.com/afreidah/flight-fetcher/internal/apiclient/dump1090"
	"github.com/afreidah/flight-fetcher/internal/apiclient/flightaware"
	"github.com/afreidah/flight-fetcher/internal/apiclient/hexdb"
	"github.com/afreidah/flight-fetcher/internal/apiclient/opensky"
	"github.com/afreidah/flight-fetcher/internal/config"
	"github.com/afreidah/flight-fetcher/internal/enricher"
	"github.com/afreidah/flight-fetcher/internal/geo"
	"github.com/afreidah/flight-fetcher/internal/notify"
	"github.com/afreidah/flight-fetcher/internal/notify/discord"
	"github.com/afreidah/flight-fetcher/internal/notify/telegram"
	"github.com/afreidah/flight-fetcher/internal/route"
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// stateFetcher is the entrypoint's view of a poller data source. main is the
// composition root and the one place the OpenSky and dump1090 clients are held
// in the same slice, so the interface that unifies them is declared here
// rather than exported by the poller.
type stateFetcher interface {
	GetStates(ctx context.Context, bbox geo.BBox) (*opensky.StatesResponse, error)
}

// sourceSpec is a planned poller: the name it reports in logs and metrics, the
// client it polls, and the interval resolved from per-source config falling
// back to the top-level default.
type sourceSpec struct {
	name     string
	source   stateFetcher
	interval time.Duration
}

// notifierSpec is a planned notification backend: the name reported in logs
// and the client that delivers to it.
type notifierSpec struct {
	name     string
	notifier notify.Notifier
}

// -------------------------------------------------------------------------
// PLANNERS
// -------------------------------------------------------------------------

// firstNonZeroInterval returns the first interval that is > 0. It lets a
// per-source interval override the top-level default without requiring
// callers to handle zero values.
func firstNonZeroInterval(intervals ...time.Duration) time.Duration {
	for _, d := range intervals {
		if d > 0 {
			return d
		}
	}
	return 0
}

// plannedSources returns the flight sources to poll, in priority order, with
// each interval resolved from its own block falling back to the top-level
// default. OpenSky is always present; the local antenna is appended only when
// a dump1090 block is configured.
//
// The OpenSky client built here is one of three the service constructs, the
// others being for metadata lookup and for the squawk monitor. That is
// deliberate: an opensky.Client owns its own token cache and its own backoff
// window, so a 429 on one workload does not silence the others. Emergency
// squawk detection in particular must not stop because routine polling hit a
// rate limit. The cost is that the account-level credit budget is approached
// by three independent backoff states rather than one; issue #159 tracks
// moving backoff into apiclient per endpoint, which would let these share a
// client without sharing a stall.
func plannedSources(cfg *config.Config) []sourceSpec {
	specs := []sourceSpec{{
		name:     "opensky",
		source:   opensky.NewClient(cfg.OpenSky.ID, cfg.OpenSky.Secret),
		interval: firstNonZeroInterval(cfg.OpenSky.Interval, cfg.Poll),
	}}
	if cfg.Dump1090 != nil {
		specs = append(specs, sourceSpec{
			name:     "antenna",
			source:   dump1090.NewClient(cfg.Dump1090.URL),
			interval: firstNonZeroInterval(cfg.Dump1090.Interval, cfg.Poll),
		})
	}
	return specs
}

// sourceNames returns the names of the planned sources, which the dashboard
// needs in order to report which source is currently hearing each aircraft.
func sourceNames(specs []sourceSpec) []string {
	names := make([]string, 0, len(specs))
	for _, s := range specs {
		names = append(names, s.name)
	}
	return names
}

// cacheTTL returns the slowest of the planned poll intervals and the Redis TTL
// derived from it. The TTL must cover the slowest poller's cycle or aircraft
// disappear from the dashboard between polls; three cycles gives two grace
// periods before eviction.
func cacheTTL(specs []sourceSpec) (slowest, ttl time.Duration) {
	for _, s := range specs {
		if s.interval > slowest {
			slowest = s.interval
		}
	}
	return slowest, slowest * 3
}

// plannedAircraftSources returns the aircraft metadata lookups to try, in
// order. HexDB is unauthenticated so it goes first; the OpenSky lookup behind
// it spends credits and gets its own client, for the reason given on
// plannedSources.
func plannedAircraftSources(cfg *config.Config, images *hexdb.Client) []enricher.NamedSource[aircraft.Info] {
	return []enricher.NamedSource[aircraft.Info]{
		{Name: "hexdb", Fn: images.Lookup},
		{Name: "opensky", Fn: opensky.NewClient(cfg.OpenSky.ID, cfg.OpenSky.Secret).Lookup},
	}
}

// plannedRouteSources returns the route lookups to try, in order. Both are
// optional and both are metered, so AirLabs goes first and FlightAware is only
// reached for callsigns it could not resolve. An empty result disables route
// enrichment entirely.
func plannedRouteSources(cfg *config.Config) []enricher.NamedSource[route.Info] {
	var sources []enricher.NamedSource[route.Info]
	if cfg.AirLabs != nil && cfg.AirLabs.APIKey != "" {
		sources = append(sources, enricher.NamedSource[route.Info]{
			Name: "airlabs", Fn: airlabs.NewClient(cfg.AirLabs.APIKey).LookupRoute,
		})
	}
	if cfg.FlightAware != nil && cfg.FlightAware.APIKey != "" {
		sources = append(sources, enricher.NamedSource[route.Info]{
			Name: "flightaware", Fn: flightaware.NewClient(cfg.FlightAware.APIKey).LookupRoute,
		})
	}
	return sources
}

// plannedNotifiers returns the notification backends to register, one per
// configured block. An empty result leaves the manager a no-op, which is how
// the squawk monitor runs with detection and storage but no alerting.
func plannedNotifiers(cfg *config.Config) []notifierSpec {
	if cfg.Notifications == nil {
		return nil
	}
	var specs []notifierSpec
	for _, d := range cfg.Notifications.Discord {
		specs = append(specs, notifierSpec{name: "discord", notifier: discord.New(d.WebhookURL)})
	}
	for _, t := range cfg.Notifications.Telegram {
		specs = append(specs, notifierSpec{name: "telegram", notifier: telegram.New(t.BotToken, t.ChatID)})
	}
	return specs
}
