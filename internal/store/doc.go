// -------------------------------------------------------------------------------
// Store - Package Documentation
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Package-level documentation and layering notes for the store package.
// -------------------------------------------------------------------------------

// Package store is the persistence layer, split across two backends chosen for
// two different lifetimes of data.
//
// RedisStore holds current flight state. Each aircraft is keyed by ICAO24 with
// a TTL, so an aircraft that leaves the area or stops broadcasting expires on
// its own and no sweeper is needed. It also records per-source heard markers,
// which is what lets the dashboard show which poller is currently hearing a
// given aircraft. The TTL is derived by the entrypoint from the slowest
// configured poll interval; set below that, aircraft flicker out between polls.
//
// PostgresStore holds everything meant to outlive a poll cycle: the aircraft
// metadata and route caches that keep enrichment from re-querying paid APIs,
// the historical sighting log, and squawk alerts. Queries are sqlc-generated
// against the schema in the migrations subpackage and run over a pgx pool;
// goose migrations are applied on startup. Row types are mapped to the
// aircraft, route, and squawk domain types at this boundary so no sqlc type
// escapes the package.
//
// Both stores satisfy the narrow interfaces their consumers declare, and
// neither is depended on by name above this layer. Only the composition root
// in cmd/server holds the concrete types.
//
// store sits above the domain and transport layers, importing aircraft, route,
// squawk, and opensky for the state vector type. It is consumed by the
// enricher, poller, retention worker, and server.
package store
