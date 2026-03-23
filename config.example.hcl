# -------------------------------------------------------------------------------
# Flight Fetcher - Example Configuration
#
# Project: Flight Fetcher / Author: Alex Freidah
#
# Copy this file to config.hcl and fill in your OpenSky Network credentials.
# Register at https://opensky-network.org to get free API access.
# -------------------------------------------------------------------------------

location {
  lat       = 34.0928
  lon       = -118.3287
  radius_km = 50.0
}

opensky {
  id     = "YOUR_CLIENT_ID"
  secret = "YOUR_CLIENT_SECRET"
}

poll_interval = "20s"

redis {
  addr = "redis:6379"
}

postgres {
  dsn = "postgres://flight_fetcher:flight_fetcher@postgres:5432/flight_fetcher?sslmode=disable"
}

server {
  listen = ":8080"
}
