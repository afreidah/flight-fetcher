# -------------------------------------------------------------------------------
# Flight Fetcher - Nomad Job Specification
#
# Project: Flight Fetcher / Author: Alex Freidah
#
# Polls OpenSky Network for aircraft within a configurable radius, enriches
# metadata via HexDB.io and AirLabs/FlightAware, stores current state in Redis
# and history in Postgres. Secrets are injected via Vault template. Includes
# health checking, rolling updates, and resource constraints.
# -------------------------------------------------------------------------------

job "flight-fetcher" {
  datacenters = ["dc1"]
  type        = "service"

  # -------------------------------------------------------------------------
  # UPDATE STRATEGY
  # -------------------------------------------------------------------------

  update {
    min_healthy_time = "10s"
    healthy_deadline = "2m"
    auto_revert      = true
  }

  # -------------------------------------------------------------------------
  # TASK GROUP
  # -------------------------------------------------------------------------

  group "flight-fetcher" {
    count = 1

    # --- Network configuration ---
    network {
      mode = "bridge"
      port "http" {
        to = 8080
      }
    }

    # --- Service registration with health check ---
    service {
      name     = "flight-fetcher"
      port     = "http"
      provider = "consul"

      check {
        type     = "http"
        path     = "/healthz"
        interval = "10s"
        timeout  = "2s"
      }
    }

    # -----------------------------------------------------------------------
    # Task: server
    # -----------------------------------------------------------------------

    task "server" {
      driver = "docker"

      vault {
        role = "flight-fetcher"
      }

      identity {
        env  = true
        file = true
        aud  = ["vault.io"]
      }

      # --- Container configuration ---
      config {
        image = "flight-fetcher:latest"
        args  = ["-config", "/secrets/config.hcl", "-log-level", "info"]
        ports = ["http"]
      }

      # --- Configuration rendered by Vault ---
      template {
        data        = <<EOH
{{ with secret "secret/data/flight-fetcher" }}
poll_interval      = "20s"
enrichment_refresh = "1h"

location {
  lat       = 0.0
  lon       = 0.0
  radius_km = 50.0
}

opensky {
  id     = "{{ .Data.data.opensky_id }}"
  secret = "{{ .Data.data.opensky_secret }}"
}

redis {
  addr = "redis.service.consul:6379"
}

postgres {
  dsn = "postgres://flight_fetcher:{{ .Data.data.db_password }}@haproxy-postgres.service.consul:5433/flight_fetcher?sslmode=require"
}

server {
  listen  = ":8080"
  refresh = 5
}

airlabs {
  api_key = "{{ .Data.data.airlabs_api_key }}"
}

squawk_monitor {
  interval = "60s"
}

retention {
  sightings_max_age = "720h"
  alerts_max_age    = "168h"
  routes_max_age    = "24h"
}
{{ end }}
EOH
        destination = "secrets/config.hcl"
        change_mode = "restart"
      }

      # --- OTel collector endpoint (optional) ---
      env {
        OTEL_EXPORTER_OTLP_ENDPOINT = "http://otel-collector.service.consul:4317"
      }

      # --- Resources ---
      resources {
        cpu    = 200
        memory = 128
      }
    }
  }
}
