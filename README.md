![Dashboard](docs/webui.png)

# Flight Fetcher

[![CI](https://github.com/afreidah/flight-fetcher/actions/workflows/ci.yml/badge.svg)](https://github.com/afreidah/flight-fetcher/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/afreidah/flight-fetcher/branch/main/graph/badge.svg)](https://codecov.io/gh/afreidah/flight-fetcher)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A self-hosted aircraft tracking service written in Go that monitors airspace around a configurable location in real time. The service polls the OpenSky Network API for aircraft within a given radius, enriches each flight with metadata and route information from multiple sources, monitors for global emergency squawk codes, and serves a live web dashboard with an interactive map.

## Core Functionality

* **Aircraft Polling and Filtering** - Queries the OpenSky Network REST API on a configurable interval for aircraft state vectors within a geographic bounding box, then applies precise haversine distance filtering to enforce a circular radius. Each poll cycle captures ICAO24 identifier, callsign, position, altitude, velocity, heading, vertical rate, ground status, and squawk transponder code.

* **Aircraft Metadata Enrichment** - When a previously unseen ICAO24 appears, the service queries HexDB.io for static aircraft information including registration number, manufacturer, aircraft type, and operator. Results are cached in PostgreSQL so each aircraft is only looked up once. Enrichment is retried on transient errors.

* **Flight Route Enrichment** - When a new callsign appears, the service queries AirLabs (primary) and FlightAware AeroAPI (fallback) to resolve departure and arrival airports. Routes are cached in PostgreSQL with a configurable TTL (default 24h) so stale routes are refreshed daily. The enrichment cache is periodically evicted to retry previously failed lookups.

* **Squawk Code Tracking** - Parses transponder squawk codes from OpenSky data for all local aircraft. A separate background worker optionally polls the global OpenSky endpoint to detect emergency squawk codes worldwide (7500 hijack, 7600 radio failure, 7700 general emergency), with in-memory deduplication to prevent duplicate alerts.

* **Dual Storage** - Current flight state is written to Redis with a TTL of 3x the poll interval, so aircraft automatically disappear when they leave the area. Historical sightings, aircraft metadata, flight routes, and squawk alerts are persisted in PostgreSQL via sqlc-generated queries, with goose migrations run automatically on startup.

* **Data Retention** - An optional background worker periodically cleans up old sightings, squawk alerts, and stale routes to prevent unbounded table growth.

* **Web Dashboard** - A split-pane dashboard with an interactive map. The left pane shows flight cards and global squawk alerts. The right pane has a persistent Leaflet map with all aircraft plotted as rotated airplane icons, with flight detail rendered below. Clicking a card or marker selects the flight, highlights it on the map, and displays enriched metadata and route information. Refresh interval is configurable.

```
         OpenSky Network API
                  |
         poll on interval
                  |
         +--------v---------+
         |  flight-fetcher  |---> HexDB.io (aircraft metadata)
         |                  |---> AirLabs (flight routes, primary)
         |                  |---> FlightAware (flight routes, fallback)
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
# Register at https://flightaware.com/aeroapi for route fallback (optional)
docker compose up --build
```

The dashboard is available at `http://localhost:8080`.

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
```

## Deployment

Deploys as a Nomad job with Consul service discovery, Vault secret injection, and Traefik reverse proxy with OAuth2 authentication. The Docker image is a multi-stage Alpine build producing a minimal container with the static Go binary. A docker-compose environment is provided for local development with PostgreSQL and Redis.

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
  config/config.go          # HCL config loading and validation
  poller/poller.go          # OpenSky polling loop with enrichment eviction
  opensky/                  # OpenSky API client, OAuth2, backoff, types
  hexdb/                    # HexDB.io API client and types
  airlabs/                  # AirLabs API client and types (route primary)
  flightaware/              # FlightAware AeroAPI client (route fallback)
  enricher/enricher.go      # aircraft metadata + route enrichment with fallback
  squawk/                   # global emergency squawk monitor
  retention/                # data retention cleanup worker
  runloop/                  # shared ticker loop helper
  server/                   # web dashboard (split-pane map, JSON API)
  store/
    redis.go                # current flight state (TTL-based)
    postgres.go             # metadata, routes, sightings, squawk alerts
  geo/geo.go                # haversine distance, bbox calculation
deploy/
  Dockerfile                # multi-stage Alpine build
```

## License

MIT
