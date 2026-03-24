# Changelog

All notable changes to this project are documented in this file.


## [unreleased]

### Added
- Add sortable flight list columns and deduplicate sentinel check
- Add route cache TTL, fix squawk alert JSON fields, update docs
- Add bounded seen maps with configurable eviction and enricher logging
- Add FlightAware AeroAPI as fallback route lookup
- Add config validation on startup
- Add data retention worker and fix OpenSky OAuth2 authentication
- Add HTTP client timeouts to all external API clients
- Add exponential backoff on OpenSky 429 rate limit responses
- Add deduplication for squawk alerts and poller enrichment
- Add global emergency squawk monitor
- Add flight route enrichment via AirLabs API
- Add web dashboard for monitoring current flights
- Add unit tests with interfaces and generated mocks
- Add docker-compose development environment
- Add sqlc for Postgres queries and goose schema migrations
- Add style guide, CI, repo baseline, and apply conventions

### Fixed
- Fix OAuth2 token fetch race condition
- Fix squawk alert selection and add temporary map marker
- Fix squawk alert row alignment and squawk label HTML rendering
- Fix dashboard flicker by updating DOM in place
- Fix enrichment return semantics to allow retry on transient errors
- Fix dashboard blank flashes between poll cycles

### Hardened
- Harden OpenSky response parsing with custom JSON unmarshaler

### Improved
- Replace positional params with Options structs on server.New and poller.New
- Replace sqlc type with domain type for squawk alerts
- updated dashboard image
- Replace Redis KEYS with SCAN in GetAllFlights
- updated dashboard ui image
- updated dashboard image
- updated dashboard pic
- Update README.md

### Other
- Extract shared route.Info domain type from airlabs package
- Cache rendered HTML page at startup instead of per-request
- Move tokenURL to client field and parse durations once at load time
- Use errors.Is for pgx.ErrNoRows checks
- Move squawk alert dedup from in-memory to Postgres
- Redesign dashboard with split-pane layout and persistent map
- Extract shared ticker loop into runloop package
- Remove route sentinel caching that blocked legitimate route data
- Show aircraft position on map in flight detail pane
- Standardize dashboard units to imperial and polish card colors
- Cache negative lookups, configurable refresh, and UI polish
- Redesign dashboard with expandable cards, version display, and new API endpoints
- twaks
- Parse and display squawk codes from OpenSky data
- Initial skeleton: flight tracking poller service
