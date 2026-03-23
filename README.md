# Flight Fetcher

![Dashboard](docs/webui.png)

[![CI](https://github.com/afreidah/flight-fetcher/actions/workflows/ci.yml/badge.svg)](https://github.com/afreidah/flight-fetcher/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/afreidah/flight-fetcher/branch/main/graph/badge.svg)](https://codecov.io/gh/afreidah/flight-fetcher)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A Go service that polls the OpenSky Network API for aircraft within a configurable radius of a fixed location, enriches aircraft metadata via HexDB.io, looks up flight routes via AirLabs, and serves a live web dashboard.

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
