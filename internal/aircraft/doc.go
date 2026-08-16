// -------------------------------------------------------------------------------
// Aircraft - Package Documentation
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Package-level documentation and layering notes for the aircraft package.
// -------------------------------------------------------------------------------

// Package aircraft holds the aircraft metadata domain type and the static
// reference data used to describe an airframe: classification as military, law
// enforcement, or emergency services from the ICAO24 hex range and registered
// owner, type specifications keyed by ICAO type code, and airline details keyed
// by operator flag code. Reference data comes from tar1090-db (military hex
// ranges, aircraft types) and OpenFlights (airline codes), embedded at build
// time so no lookup requires a network call.
//
// Info is the type every layer agrees on. API clients decode their own wire
// formats into it, the store persists it, and the server serves it, which keeps
// a change to one upstream provider from reaching the rest of the pipeline. A
// sentinel Info - one carrying only an ICAO24 - records that enrichment ran and
// found nothing, so consumers must call IsSentinel before treating a non-nil
// result as real metadata.
//
// aircraft is a domain package. It imports nothing from this module; the
// apiclient, store, enricher, and server layers all import it.
package aircraft
