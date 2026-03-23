# Flight Fetcher

[![CI](https://github.com/afreidah/flight-fetcher/actions/workflows/ci.yml/badge.svg)](https://github.com/afreidah/flight-fetcher/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/afreidah/flight-fetcher/branch/main/graph/badge.svg)](https://codecov.io/gh/afreidah/flight-fetcher)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A Go service that polls the OpenSky Network API for aircraft within a configurable radius of a fixed location, enriches aircraft metadata via HexDB.io, and stores results for later consumption.

```
         OpenSky Network API
                  |
          poll every ~20s
                  |
         +--------v---------+
         |  flight-fetcher  |---> HexDB.io (enrich new ICAO24s)
         +---+---------+----+
             |         |
    +--------v--+  +---v--------+
    |   Redis   |  | PostgreSQL |
    | (current  |  | (metadata  |
    |  state)   |  |  + history)|
    +-----------+  +------------+
```

## Configuration

Configuration is loaded from an HCL file. Secrets are templated in by Vault at deploy time.

```hcl
location {
  lat       = 40.0
  lon       = -74.0
  radius_km = 50.0
}

opensky {
  username = "user"
  password = "pass"
}

poll_interval = "20s"

redis {
  addr = "redis.service.consul:6379"
}

postgres {
  dsn = "postgres://user:pass@host:5432/flight_fetcher?sslmode=require"
}
```

## Deployment

Deploys as a Nomad job with Vault integration for secret injection. The Nomad template renders the full config file with OpenSky credentials and database password from Vault.

## Development

```bash
make help                   # show all targets
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
  opensky/                  # OpenSky API client and types
  hexdb/                    # HexDB.io API client and types
  enricher/enricher.go      # aircraft metadata enrichment
  store/
    redis.go                # current flight state (TTL-based)
    postgres.go             # aircraft metadata cache + sighting history
  geo/geo.go                # haversine distance, bbox calculation
deploy/
  Dockerfile                # multi-stage Alpine build
  nomad.hcl                 # Nomad job with Vault template
```

## License

MIT
