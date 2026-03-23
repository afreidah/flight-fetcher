job "flight-fetcher" {
  datacenters = ["dc1"]
  type        = "service"

  group "flight-fetcher" {
    count = 1

    network {
      mode = "bridge"
    }

    service {
      name     = "flight-fetcher"
      provider = "consul"
    }

    vault {
      policies = ["flight-fetcher"]
    }

    task "server" {
      driver = "docker"

      config {
        image = "flight-fetcher:latest"
        args  = ["-config", "/local/config.hcl"]
      }

      template {
        data        = <<-EOF
        {{ with secret "secret/data/flight-fetcher" }}
        OPENSKY_USERNAME={{ .Data.data.opensky_username }}
        OPENSKY_PASSWORD={{ .Data.data.opensky_password }}
        {{ end }}
        EOF
        destination = "secrets/env"
        env         = true
      }

      template {
        data        = file("config.hcl")
        destination = "local/config.hcl"
      }

      resources {
        cpu    = 100
        memory = 64
      }
    }
  }
}
