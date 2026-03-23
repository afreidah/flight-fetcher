# -------------------------------------------------------------------------------
# Flight Fetcher - Container Build
#
# Project: Flight Fetcher / Author: Alex Freidah
#
# Multi-stage build producing a minimal Alpine image with the flight-fetcher
# binary. Configuration is mounted at runtime by Nomad.
# -------------------------------------------------------------------------------

FROM golang:1.26-alpine AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.Version=${VERSION}" -o /flight-fetcher ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=build /flight-fetcher /usr/local/bin/flight-fetcher
ENTRYPOINT ["flight-fetcher"]
