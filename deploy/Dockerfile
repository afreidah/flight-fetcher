# -------------------------------------------------------------------------------
# Flight Fetcher - Container Build
#
# Project: Flight Fetcher / Author: Alex Freidah
#
# Multi-stage cross-compiled build. The build stage pins itself to the host
# architecture via --platform=$BUILDPLATFORM and uses Go's native cross-
# compiler (GOOS/GOARCH from $TARGETOS/$TARGETARCH) to produce the binary
# for the target platform. This avoids QEMU emulation for the compile step,
# dropping arm64 push time from ~12min to ~45s on an amd64 host.
#
# Configuration is mounted at runtime by Nomad.
# -------------------------------------------------------------------------------

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w -X main.Version=${VERSION}" -o /flight-fetcher ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates && \
    adduser -D -u 1000 appuser
COPY --from=build /flight-fetcher /usr/local/bin/flight-fetcher
USER appuser
ENTRYPOINT ["flight-fetcher"]
