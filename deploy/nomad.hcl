# -------------------------------------------------------------------------------
# Flight Fetcher - Nomad Job Specification
#
# Project: Flight Fetcher / Author: Alex Freidah
#
# Polls OpenSky Network for aircraft within a configurable radius, enriches
# metadata via HexDB.io, stores current state in Redis and history in Postgres.
# OpenSky credentials are injected into the config file via Vault template.
# -------------------------------------------------------------------------------

job "flight-fetcher" {
  datacenters = ["dc1"]
  type        = "service"

  # -------------------------------------------------------------------------
  # TASK GROUP
  # -------------------------------------------------------------------------

  group "flight-fetcher" {
    count = 1

    # --- Network configuration ---
    network {
      mode = "bridge"
    }

    # --- Service registration ---
    service {
      name     = "flight-fetcher"
      provider = "consul"
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
        args  = ["-config", "/secrets/config.hcl"]
      }

      # --- Configuration rendered by Vault ---
      template {
        data        = <<EOH
{{ with secret "secret/data/flight-fetcher" }}
location {
  lat       = 0.0
  lon       = 0.0
  radius_km = 50.0
}

opensky {
  username = "{{ .Data.data.opensky_username }}"
  password = "{{ .Data.data.opensky_password }}"
}

poll_interval = "20s"

redis {
  addr = "redis.service.consul:6379"
}

postgres {
  dsn = "postgres://flight_fetcher:{{ .Data.data.db_password }}@haproxy-postgres.service.consul:5433/flight_fetcher?sslmode=require"
}
{{ end }}
EOH
        destination = "secrets/config.hcl"
        change_mode = "restart"
      }

      # --- Resources ---
      resources {
        cpu    = 100
        memory = 64
      }
    }
  }
}
