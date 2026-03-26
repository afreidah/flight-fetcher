# -------------------------------------------------------------------------------
# Flight Fetcher - Example Configuration
#
# Project: Flight Fetcher / Author: Alex Freidah
#
# Copy this file to config.hcl and fill in your credentials.
# Register at https://opensky-network.org for OpenSky API access.
# Register at https://airlabs.co for AirLabs flight route data (optional).
# Register at https://flightaware.com/aeroapi for FlightAware fallback (optional).
# -------------------------------------------------------------------------------

poll_interval      = "20s"
enrichment_refresh = "1h"

location {
  lat       = 34.0928
  lon       = -118.3287
  radius_km = 50.0
}

opensky {
  id     = "YOUR_CLIENT_ID"
  secret = "YOUR_CLIENT_SECRET"
}

redis {
  addr = "redis:6379"
}

postgres {
  dsn = "postgres://flight_fetcher:flight_fetcher@postgres:5432/flight_fetcher?sslmode=disable"
}

server {
  listen  = ":8080"
  refresh = 5
}

airlabs {
  api_key = "YOUR_API_KEY"
}

# Optional: FlightAware AeroAPI as fallback for route lookups (500 req/month free)
flightaware {
  api_key = "YOUR_API_KEY"
}

# Optional: global emergency squawk monitoring (7500/7600/7700)
squawk_monitor {
  interval = "60s"
}

# Optional: notification backends for emergency squawk alerts.
# Multiple blocks of the same type are supported (e.g., two Discord webhooks).
notifications {
  discord {
    webhook_url = "https://discord.com/api/webhooks/YOUR_WEBHOOK_ID/YOUR_WEBHOOK_TOKEN"
  }

  # telegram {
  #   bot_token = "YOUR_BOT_TOKEN"
  #   chat_id   = "YOUR_CHAT_ID"
  # }
}

# Optional: automatic cleanup of old data
# Optional: local ADS-B receiver (dump1090/readsb/dump1090-fa)
# dump1090 {
#   url = "http://piaware:8080"
# }

retention {
  sightings_max_age = "720h"
  alerts_max_age    = "168h"
  routes_max_age    = "24h"
}
