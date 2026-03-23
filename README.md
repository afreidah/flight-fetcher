![Dashboard](docs/webui.png)

# Flight Fetcher

[![CI](https://github.com/afreidah/flight-fetcher/actions/workflows/ci.yml/badge.svg)](https://github.com/afreidah/flight-fetcher/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/afreidah/flight-fetcher/branch/main/graph/badge.svg)](https://codecov.io/gh/afreidah/flight-fetcher)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A self-hosted aircraft tracking service written in Go that monitors airspace around a configurable location in real time. The service polls the OpenSky Network API every 20 seconds for aircraft within a given radius, enriches each flight with metadata and route information from multiple sources, and serves a live web dashboard for visualization.

## Core Functionality

* **Aircraft Polling and Filtering** - Queries the OpenSky Network REST API for aircraft state vectors within a geographic bounding box, then applies precise haversine distance filtering to enforce a circular radius around the configured center point. Each poll cycle captures ICAO24 identifier, callsign, position, altitude, velocity, heading, vertical rate, ground status, and squawk transponder code.

* **Aircraft Metadata Enrichment** - When a previously unseen ICAO24 appears, the service queries HexDB.io for static aircraft information including registration number, manufacturer, aircraft type, and operator. Results are cached in PostgreSQL so each aircraft is only looked up once.

* **Flight Route Enrichment** - When a new callsign appears, the service queries the AirLabs API to resolve departure and arrival airports (IATA/ICAO codes and full airport names). Routes are cached in PostgreSQL to minimize usage against the free API tier (1,000 requests/month). Empty and whitespace-only callsigns are skipped.

* **Squawk Code Tracking** - Parses transponder squawk codes from OpenSky data for all local aircraft. A separate background worker optionally polls the global OpenSky endpoint on a configurable interval to detect emergency squawk codes worldwide (7500 hijack, 7600 radio failure, 7700 general emergency), enriches matching aircraft, and stores them for dashboard display.

* **Dual Storage** - Current flight state is written to Redis with a 2-minute TTL, so aircraft automatically disappear when they leave the area or stop broadcasting. Historical sightings, aircraft metadata, and flight routes are persisted in PostgreSQL via sqlc-generated queries, with goose migrations run automatically on startup.

* **Web Dashboard** - An embedded HTML dashboard served on a configurable HTTP port provides a live view of all tracked aircraft with auto-refresh every 5 seconds. Flights are displayed as clickable cards showing ICAO24, callsign, country, altitude, speed, and squawk code. Clicking a flight reveals full detail including position, flight state, departure/arrival airports, aircraft metadata, and squawk status. Emergency squawk codes are highlighted in red. A global squawk alerts section surfaces emergency transponder events from around the world.

## Architecture

  - Configuration via HCL files with optional blocks for the dashboard server, AirLabs API, and squawk monitor. Secrets are templated by Vault in production.
  - Dependency injection via interfaces at the consumer, with gomock-generated mocks for unit testing.
  - Structured logging via log/slog with JSON output and context propagation throughout.
  - Resilient error handling where individual item failures (cache writes, sighting logs, enrichment lookups) are logged and skipped without stopping the batch.
  - Graceful shutdown via signal handling with context cancellation propagated to all components.

Deployment

Deploys as a Nomad job with Consul service discovery, Vault secret injection, and Traefik reverse proxy with OAuth2 authentication. The Docker image is a multi-stage Alpine build producing a minimal container with the static Go binary. A docker-compose environment is provided for local development with PostgreSQL and Redis.


```
         OpenSky Network API
                  |
          poll every ~20s
                  |
         +--------v---------+
         |  flight-fetcher  |---> HexDB.io (aircraft metadata)
         |                  |---> AirLabs (flight routes)
         +--+---------+--+--+
            |         |  |
   +--------v--+  +---v--v-----+    +-------------+
   |   Redis   |  | PostgreSQL |    |  Dashboard  |
   | (current  |  | (metadata  |    |  :8080      |
   |  state)   |  |  + routes  |    +-------------+
   +-----------+  |  + history)|
                  +------------+
```

## Quick Start

```bash
cp config.example.hcl config.hcl
# Edit config.hcl with your credentials
# Register at https://opensky-network.org for OpenSky API access
# Register at https://airlabs.co for flight route data (optional)
docker compose up --build
```

The service starts polling OpenSky for aircraft near the configured location, enriches metadata via HexDB.io, looks up departure/arrival airports via AirLabs, and stores results in Postgres and Redis. The dashboard is available at `http://localhost:8080`.

## Configuration

Configuration is loaded from an HCL file. Secrets are templated in by Vault at deploy time.

```hcl
location {
  lat       = 40.0
  lon       = -74.0
  radius_km = 50.0
}

opensky {
  id     = "client_id"
  secret = "client_secret"
}

poll_interval = "20s"

redis {
  addr = "redis.service.consul:6379"
}

postgres {
  dsn = "postgres://user:pass@host:5432/flight_fetcher?sslmode=require"
}

# Optional: web dashboard
server {
  listen = ":8080"
}

# Optional: flight route enrichment (free tier: 1,000 req/month)
airlabs {
  api_key = "your_api_key"
}
```

## Deployment

Deploys as a Nomad job with Vault integration for secret injection. The Nomad template renders the full config file with API credentials and database passwords from Vault.

## Development

```bash
make help                   # show all targets
make build                  # build the binary locally
make vet                    # Go vet static analysis
make lint                   # golangci-lint
make test                   # unit tests with race detector and coverage
make govulncheck            # Go vulnerability scanner
make run                    # run locally (requires config.hcl)
make push                   # build and push multi-arch images to registry
```

## Project Structure

```
cmd/
  server/
    main.go                 # entrypoint, config, signal handling
internal/
  config/config.go          # HCL config loading
  poller/poller.go          # OpenSky polling loop
  opensky/                  # OpenSky API client and types (incl. squawk codes)
  hexdb/                    # HexDB.io API client and types
  airlabs/                  # AirLabs API client and types (flight routes)
  enricher/enricher.go      # aircraft metadata + route enrichment
  server/                   # web dashboard (embedded HTML, JSON API)
  store/
    redis.go                # current flight state (TTL-based)
    postgres.go             # aircraft metadata, routes, sighting history
  geo/geo.go                # haversine distance, bbox calculation
deploy/
  Dockerfile                # multi-stage Alpine build
```

## License

MIT
