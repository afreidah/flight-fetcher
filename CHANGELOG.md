# Changelog

All notable changes to this project are documented in this file.


## [0.9.25] - 2026-04-20

### Fixed
- Fix multi-poller TTL and add Antenna Live toggle

### Improved
- update CHANGELOG.md for v0.9.23 (#201)

### Other
- Cross-compile for arm64 in Dockerfile to kill QEMU build cost
- Cover dual-source additions with tests
- Derive origin_country locally for dump1090 aircraft
- Run multiple flight-source pollers concurrently
- Surface dump1090 antenna fields in state vector and dashboard

## [0.9.23] - 2026-03-30

### Added
- Add dump1090/readsb local ADS-B receiver client
- Add tests for LookupType, DescribeAircraftClass, LookupAirline, and flight list classification
- Add type specs, airline details, logos, and classification in flight list
- Add Telegram notifier and restructure notification config
- Add pluggable notification system with Discord webhook support
- Add tracing spans to all Postgres operations and fix observe shutdown
- Add generic Lookup helper to reduce API client boilerplate
- Add config validation tests for missing coverage branches
- Add integration tests for Postgres and Redis store layer
- Add Grafana dashboard and local observability stack
- Add named enrichment sources and improve log visibility
- Add OpenSky metadata as aircraft enrichment fallback

### Fixed
- Fix squawk detail image layout to match flight detail
- Fix route seen-marking, remove sync image fetch, async squawk enrichment

### Improved
- update CHANGELOG.md for v0.9.4 (#142)

### Other
- Deduplicate sighting writes and add aircraft metadata TTL
- Remove unfinished TFR code from dump1090 branch
- Classify aircraft as military/LE/EMS and fix image rendering
- Enrich aircraft with type code, owner, and photo from HexDB
- Thread migration context, extract squawk constants, parse Retry-After
- Deduplicate Do() metrics, remove hexdb type alias, normalize JSON casing
- Clean up config, server Pinger, and ListenAndServe error handling
- Unify duplicate enricher interfaces into enricher.Interface
- Deduplicate enricher with generic lookup helper
- Fill test coverage gaps and include store in codecov
- Upload integration test coverage to codecov

## [0.9.4] - 2026-03-25

### Fixed
- fixup: make release task work

### Other
- Nest API client packages under apiclient/

## [0.9.2] - 2026-03-25

### Added
- Add GoReleaser config and release workflow
- Add unit tests for aircraft domain type
- Add unit tests for observe package
- Add OpenTelemetry tracing, Prometheus metrics, and log correlation
- Add database indexes, updated_at column, and batched retention deletes
- Add circuit breaking and graceful degradation
- Add connection pool configuration and configurable log level
- Add graceful shutdown, health check endpoint, and Redis startup ping
- Add unit tests for apiclient package
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
- Fix Nomad template, enforce poll minimum, harden Docker, simplify squawk
- Fix Redis N+1 queries, silent errors, and error comparison
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
- Extract aircraft domain type, update Nomad job, refresh README
- Decouple enrichment from poll loop with async worker pool
- Use slog.Group and errors.Join for modern Go idioms
- Extract shared apiclient and harden HTTP layer
- Sanitize API inputs and HTML outputs
- Open source prep: Options struct, CONTRIBUTING, changelog, cleanup
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
