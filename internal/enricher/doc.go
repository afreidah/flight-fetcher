// -------------------------------------------------------------------------------
// Enricher - Package Documentation
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Package-level documentation and layering notes for the enricher package.
// -------------------------------------------------------------------------------

// Package enricher resolves aircraft metadata for newly seen ICAO24 codes and
// flight routes for newly seen callsigns, checking the Postgres cache before
// calling any external API and writing results back for future lookups.
//
// Both paths run the same generic pipeline: check the cache, try each
// configured source in order until one answers, then persist. Sources are
// ordered cheapest-first, so an unauthenticated lookup is always attempted
// before one that spends credits or money.
//
// The distinction the API turns on is transient failure versus confirmed
// absence. A lookup that errors returns ok=false and the caller retries later;
// a lookup where every source answered and none had data is a completed
// enrichment with found=false. For aircraft metadata that outcome is recorded
// as a sentinel row, so an airframe missing from every registry is not looked
// up again on every sighting. Consumers must treat a sentinel as absent rather
// than rendering its empty fields.
//
// Enrichment is called from the poller's worker pool and from the squawk
// monitor, both off the critical path, because a slow external API must never
// delay a poll cycle or an emergency detection.
//
// enricher sits in the orchestration layer. It imports the aircraft and route
// domain types and declares narrow views of its persistence and lookup
// dependencies rather than importing store or any provider client.
package enricher
