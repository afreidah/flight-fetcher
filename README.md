![Dashboard](docs/webui.png)

# Flight Fetcher

[![CI](https://github.com/afreidah/flight-fetcher/actions/workflows/ci.yml/badge.svg)](https://github.com/afreidah/flight-fetcher/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/afreidah/flight-fetcher/branch/main/graph/badge.svg)](https://codecov.io/gh/afreidah/flight-fetcher)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A self-hosted aircraft tracking service written in Go that monitors airspace around a configurable location in real time. The service polls the OpenSky Network API for aircraft within a given radius, enriches each flight with metadata and route information from multiple sources, monitors for global emergency squawk codes, and serves a live web dashboard with an interactive map.

## Core Functionality

* **Aircraft Polling and Filtering** - Queries the OpenSky Network REST API on a configurable interval for aircraft state vectors within a geographic bounding box, then applies precise haversine distance filtering to enforce a circular radius. Each poll cycle captures ICAO24 identifier, callsign, position, altitude, velocity, heading, vertical rate, ground status, and squawk transponder code.

* **Aircraft Metadata Enrichment** - When a previously unseen ICAO24 appears, the service queries a chain of sources for static aircraft information including registration number, manufacturer, aircraft type, and operator: HexDB.io (primary) with OpenSky Network metadata (fallback). Each source is tried in order until one returns data. Results are cached in PostgreSQL so each aircraft is only looked up once. Enrichment runs asynchronously in a background worker pool so poll cycles are never blocked by external API latency.

* **Flight Route Enrichment** - When a new callsign appears, the service queries a chain of sources to resolve departure and arrival airports: AirLabs (primary) with FlightAware AeroAPI (fallback). Routes are cached in PostgreSQL with a configurable TTL (default 24h) so stale routes are refreshed daily. The enrichment cache is periodically evicted to retry previously failed lookups.

* **Circuit Breaking** - All external API clients share a common HTTP client with exponential backoff on rate limits (429), server errors (5xx), and transport failures. This prevents request storms against down services and allows automatic recovery.

* **Squawk Code Tracking** - Parses transponder squawk codes from OpenSky data for all local aircraft. A separate background worker optionally polls the global OpenSky endpoint to detect emergency squawk codes worldwide (7500 hijack, 7600 radio failure, 7700 general emergency), with deduplication to prevent duplicate alerts. Detected alerts can be sent to pluggable notification backends (Discord, with more planned).

* **Dual Storage** - Current flight state is written to Redis with a TTL of 3x the poll interval, so aircraft automatically disappear when they leave the area. Historical sightings, aircraft metadata, flight routes, and squawk alerts are persisted in PostgreSQL via sqlc-generated queries, with goose migrations run automatically on startup.

* **Data Retention** - An optional background worker periodically cleans up old sightings, squawk alerts, and stale routes using batched deletes to prevent table lock contention.

* **Web Dashboard** - A split-pane dashboard with an interactive map. The left pane shows flight cards and global squawk alerts. The right pane has a persistent Leaflet map with all aircraft plotted as rotated airplane icons, with flight detail rendered below. Clicking a card or marker selects the flight, highlights it on the map, and displays enriched metadata and route information. Refresh interval is configurable.

* **Observability** - OpenTelemetry distributed tracing with OTLP gRPC export, Prometheus metrics via `/metrics` endpoint, and automatic log-trace correlation (trace_id and span_id injected into slog JSON output). Instrumentation covers API clients, poll cycles, enrichment, store operations, and HTTP requests. A pre-built Grafana dashboard is included.

* **Graceful Degradation** - The dashboard returns partial data instead of errors when backends are unavailable. The health endpoint (`/healthz`) reports per-component status with three states: healthy, degraded, and unhealthy.

```
         OpenSky Network API
                  |
         poll on interval
                  |
         +--------v---------+
         |  flight-fetcher  |---> HexDB.io (aircraft metadata, primary)
         |                  |---> OpenSky metadata (aircraft, fallback)
         |                  |---> AirLabs (flight routes, primary)
         |                  |---> FlightAware (flight routes, fallback)
         +--+---------+--+--+
            |         |  |  \
   +--------v--+  +---v--v-----+  +-------------+  +-------------+
   |   Redis   |  | PostgreSQL |  |  Dashboard  |  | Notifications|
   | (current  |  | (metadata  |  |  :8080      |  | (Discord,   |
   |  state)   |  |  + routes  |  +-------------+  |  etc.)      |
   +-----------+  |  + history)|                    +-------------+
                  +------------+
```

## Quick Start

The only prerequisite is [Docker](https://docs.docker.com/get-docker/) with Compose. Everything else — Postgres, Redis, and a full observability stack — runs in containers.

```bash
git clone https://github.com/afreidah/flight-fetcher.git
cd flight-fetcher
cp config.example.hcl config.hcl
```

Edit `config.hcl` with your API credentials:
- **OpenSky Network** (required) — register at https://opensky-network.org for a free OAuth2 client ID and secret
- **AirLabs** (optional) — register at https://airlabs.co for flight route data
- **FlightAware** (optional) — register at https://flightaware.com/aeroapi for route fallback

Then start the full stack:

```bash
make run
```

This builds the Go binary, stands up Postgres, Redis, the flight-fetcher service, and a complete observability stack (Grafana, Prometheus, Loki, Tempo, Alloy) in Docker containers:

| Service | URL | Description |
|---------|-----|-------------|
| Dashboard | http://localhost:8080 | Flight tracking web UI with interactive map |
| Grafana | http://localhost:13000 | Pre-built observability dashboard (metrics, logs, traces) |
| Prometheus | http://localhost:19090 | Metrics storage and query |

Grafana starts with anonymous admin access — no login required. The flight-fetcher dashboard is auto-imported with all datasources pre-configured. Use `make stop` to stop the stack or `make clean` to stop and remove all data.

## Configuration

Configuration is loaded from an HCL file. Secrets are templated in by Vault at deploy time.

```hcl
poll_interval      = "120s"
enrichment_refresh = "1h"

location {
  lat       = 40.0
  lon       = -74.0
  radius_km = 50.0
}

opensky {
  id     = "client_id"
  secret = "client_secret"
}

redis {
  addr = "redis:6379"
}

postgres {
  dsn = "postgres://user:pass@host:5432/flight_fetcher?sslmode=require"
}

server {
  listen  = ":8080"
  refresh = 5
}

airlabs {
  api_key = "your_api_key"
}

flightaware {
  api_key = "your_api_key"
}

squawk_monitor {
  interval = "60s"
}

retention {
  sightings_max_age = "720h"
  alerts_max_age    = "168h"
  routes_max_age    = "24h"
}

discord {
  webhook_url = "https://discord.com/api/webhooks/..."
}
```

### Required Blocks

| Block | Description |
|-------|-------------|
| `location` | Center point (`lat`, `lon`) and `radius_km` for the monitoring area |
| `opensky` | OAuth2 client credentials (`id`, `secret`) for the OpenSky Network API |
| `redis` | Redis address, optional `password` and `db` |
| `postgres` | PostgreSQL connection string (`dsn`) |

### Optional Blocks

| Block | Description |
|-------|-------------|
| `server` | HTTP dashboard and API server (`listen` address, `refresh` interval in seconds) |
| `airlabs` | AirLabs API key for flight route enrichment (primary) |
| `flightaware` | FlightAware AeroAPI key for flight route enrichment (fallback) |
| `squawk_monitor` | Global emergency squawk monitoring (`interval` between scans) |
| `retention` | Automatic cleanup of old data (`sightings_max_age`, `alerts_max_age`, `routes_max_age`) |
| `discord` | Discord webhook URL for emergency squawk notifications |

### Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | `config.hcl` | Path to configuration file |
| `-log-level` | `info` | Log verbosity: `debug`, `info`, `warn`, `error` |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP gRPC endpoint for trace export (e.g., `http://tempo:4317`). No-ops if not set or collector is unreachable. |
| `OTEL_EXPORTER_OTLP_INSECURE` | Set to `true` for unencrypted gRPC connections |

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /` | Web dashboard |
| `GET /api/flights` | All current flights (degrades to empty array if Redis is unavailable) |
| `GET /api/flights/{icao24}` | Flight detail with metadata and route |
| `GET /api/aircraft/{icao24}` | Aircraft metadata |
| `GET /api/routes/{callsign}` | Flight route |
| `GET /api/squawk-alerts` | Recent emergency squawk alerts (last 24h) |
| `GET /healthz` | Health check with per-component status (JSON) |
| `GET /metrics` | Prometheus metrics |

### Health Check Response

```json
{
  "status": "healthy",
  "components": {
    "redis": "ok",
    "postgres": "ok"
  }
}
```

Status is `healthy` (all backends up), `degraded` (some backends down, 200), or `unhealthy` (all backends down, 503).

## Observability

The service exports three pillars of observability:

### Metrics (Prometheus)

Scraped via `GET /metrics`. Key metrics:

| Metric | Type | Description |
|--------|------|-------------|
| `poller_polls_total` | counter | Poll cycles by result (`ok`, `error`) |
| `poller_poll_duration_seconds` | histogram | Poll cycle duration |
| `poller_aircraft_count` | gauge | Aircraft seen in last poll |
| `poller_enrich_queue` | gauge | Enrichment queue depth |
| `apiclient_requests_total` | counter | API requests by `upstream` and `status` |
| `apiclient_request_duration_seconds` | histogram | API request duration by `upstream` |

Plus automatic HTTP server metrics from otelhttp and Redis metrics from redisotel.

### Traces (OpenTelemetry)

Exported via OTLP gRPC. Span hierarchy:

- `poller.poll` — root span per poll cycle
  - `opensky.request` — OpenSky API call
  - `postgres.LogSighting` — sighting writes
- `hexdb.request` / `opensky.request` — enrichment API calls
  - `postgres.SaveAircraftMeta` — metadata writes
- `squawk.scan` — root span per squawk scan
- HTTP request spans — automatic via otelhttp middleware

### Logs (slog JSON)

Structured JSON to stdout. When a trace is active, `trace_id` and `span_id` are injected into every log line for correlation in Grafana.

### Grafana Dashboard

![Grafana Dashboard](docs/grafana.png)

A pre-built dashboard is included at `grafana/flight-fetcher.json` with panels for:

- Service overview (aircraft count, poll success rate, enrichment queue, API errors)
- Poll cycle duration and rate
- API client latency and error rate by upstream
- HTTP server request duration and status codes
- Recent traces (Tempo)
- Application logs with log-to-trace correlation (Loki)
- Error/warn log rate

The dashboard includes a `Log Selector` variable to switch between `service_name="flight-fetcher"` (docker compose) and `task="flight-fetcher"` (Nomad production).

## Deployment

### Docker Compose (Development)

`make run` starts the full stack including the observability stack:

| Container | Purpose |
|-----------|---------|
| `flight-fetcher` | The application |
| `postgres` | Persistent storage |
| `redis` | Flight state cache |
| `grafana` | Dashboards (port 13000) |
| `prometheus` | Metrics (port 19090) |
| `tempo` | Traces (port 13200, OTLP on 14317) |
| `loki` | Logs (port 13100) |
| `alloy` | Log collection from Docker containers |

### Nomad (Production)

Deploys as a Nomad job with Consul service discovery, Vault secret injection, and Traefik reverse proxy with OAuth2 authentication. The Docker image is a multi-stage Alpine build producing a minimal container running as a non-root user. An example Nomad job specification is included at `deploy/nomad.hcl`.

Features: rolling updates with auto-revert, Consul health check on `/healthz`, OTel trace export to Tempo, and Prometheus scraping.

## Development

```bash
make help                   # show all targets
make build                  # build the binary locally
make vet                    # Go vet static analysis
make lint                   # golangci-lint
make test                   # unit tests with race detector and coverage (skips integration)
make test-integration       # integration tests against real Postgres + Redis (requires Docker)
make govulncheck            # Go vulnerability scanner
make generate               # regenerate sqlc and mocks
make run                    # full stack with observability (requires config.hcl)
make stop                   # stop the stack
make clean                  # stop, remove volumes, remove binary
make push                   # build and push multi-arch images to registry
make release                # tag and push to trigger GitHub Release
```

## Testing

### Unit Tests

`make test` runs all unit tests with the race detector. Integration tests are automatically skipped via the `-short` flag. Unit tests use gomock for interface boundaries and httptest for API client testing.

### Integration Tests

`make test-integration` spins up real Postgres and Redis containers via [testcontainers-go](https://golang.testcontainers.org/) and runs 26 tests against them. No external API calls are made — zero credit usage.

**Postgres (18 tests):** Verifies migrations, CRUD operations, upsert behavior, TTL-aware reads (route staleness), squawk alert cooldown logic, batched retention deletes with old rows inserted via direct SQL, and connection health checks.

**Redis (8 tests):** Verifies flight state round-trips through JSON serialization, SCAN+MGET retrieval, TTL expiration, key overwriting, and connection lifecycle.

Requires Docker. Containers are started once per test run and cleaned up automatically. Each test truncates tables between runs for isolation.

### CI

The GitHub Actions CI pipeline runs unit tests and integration tests as separate jobs. Unit tests upload coverage to Codecov. Integration tests run against testcontainers with Docker available on the GitHub Actions runner.

## Project Structure

```
cmd/
  server/
    main.go                 # entrypoint, config, signal handling, errgroup
internal/
  aircraft/                 # shared aircraft metadata domain type
  apiclient/                # shared HTTP client with backoff and circuit breaking
    airlabs/                # AirLabs API client (route primary)
    flightaware/            # FlightAware AeroAPI client (route fallback)
    hexdb/                  # HexDB.io API client (aircraft metadata primary)
    opensky/                # OpenSky API client, OAuth2, metadata fallback
  config/                   # HCL config loading and validation
  enricher/                 # named source chains for metadata + route enrichment
  geo/                      # haversine distance, bbox calculation
  notify/                   # notification interface and fan-out manager
    discord/                # Discord webhook notification backend
  observe/                  # OpenTelemetry + Prometheus initialization
  poller/                   # polling loop with async enrichment worker pool
  retention/                # batched data retention cleanup worker
  route/                    # shared flight route domain type
  runloop/                  # shared ticker loop helper
  server/                   # web dashboard, JSON API, health check, metrics
  squawk/                   # global emergency squawk monitor
  store/
    redis.go                # current flight state (TTL-based, redisotel)
    postgres.go             # metadata, routes, sightings, squawk alerts
deploy/
  Dockerfile                # multi-stage Alpine build, non-root user
  nomad.hcl                 # example Nomad job specification
  monitoring/               # observability stack configs for docker compose
    tempo.yml               # Tempo trace collector
    loki.yml                # Loki log aggregator
    alloy.alloy             # Alloy log shipper (Docker container logs)
    prometheus.yml           # Prometheus scrape config
    grafana/                # Grafana provisioning (datasources, dashboards)
grafana/
  flight-fetcher.json       # pre-built Grafana dashboard
```

## License

MIT
