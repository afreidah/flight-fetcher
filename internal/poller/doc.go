// -------------------------------------------------------------------------------
// Poller - Package Documentation
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Package-level documentation and layering notes for the poller package.
// -------------------------------------------------------------------------------

// Package poller runs the main data pipeline: on each interval it queries a
// flight source for the bounding box around the configured location, discards
// aircraft outside the exact radius, writes current state and a per-source
// heard marker to the cache, logs a sighting when the aircraft has moved, and
// queues anything newly seen for enrichment.
//
// Enrichment is queued rather than performed inline, drained by a fixed worker
// pool over a buffered channel. When the queue is full the request is dropped
// with a warning: enrichment is metadata for a display, and back-pressure from
// a slow external API must never stall the poll cycle that keeps live state
// current.
//
// One Poller runs per configured source, and several may run at once - OpenSky
// for wide-area coverage alongside a local antenna. They are kept from
// duplicating work by a shared DedupState, which tracks the ICAO24 codes and
// callsigns already enriched and the last position recorded per aircraft. The
// same structure suppresses redundant sighting rows: a sighting is written only
// once an aircraft has moved past a minimum threshold, so a stationary aircraft
// on the ground does not accumulate a row per poll. Every poller must be given
// the same DedupState, and each must be given a distinct Name, which
// disambiguates its metrics, spans, and heard markers.
//
// poller sits in the orchestration layer. It imports geo and runloop and
// declares narrow views of its source, cache, sighting log, and enricher rather
// than importing the packages that supply them.
package poller
