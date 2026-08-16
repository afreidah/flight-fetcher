// -------------------------------------------------------------------------------
// Server - Package Documentation
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Package-level documentation and layering notes for the server package.
// -------------------------------------------------------------------------------

// Package server serves the web dashboard: an embedded HTML page plus the JSON
// API it reads, joining live flight state from Redis with enriched metadata,
// routes, and squawk alerts from Postgres. It also exposes /healthz and the
// Prometheus /metrics endpoint.
//
// The server is read-only. Nothing it serves is produced by it, and no handler
// writes to a store, which is why every dependency it declares is a read-side
// interface.
//
// Handlers degrade rather than fail. An unavailable cache returns an empty
// flight list instead of a 500, and metadata, route, image, and heard-by
// lookups that error are logged and omitted from the response. A dashboard
// showing live positions without registration details is useful; one showing an
// error page is not. The same applies to configuration: the optional
// dependencies are nil when their features are switched off, and the handlers
// that use them simply leave those fields out.
//
// Aircraft metadata arrives with sentinel records marking confirmed enrichment
// misses, and handlers translate those to absent rather than serving a
// structure of empty strings.
//
// server sits at the top of the orchestration layer, imported only by the
// composition root. It imports the aircraft, route, and squawk domain types and
// opensky for the state vector type, and declares narrow read-side views of its
// data sources.
package server
