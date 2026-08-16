// -------------------------------------------------------------------------------
// Route - Package Documentation
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Package-level documentation and layering notes for the route package.
// -------------------------------------------------------------------------------

// Package route holds the flight route domain type: the departure and arrival
// airports resolved for a callsign, along with the airline and flight number
// when the provider supplies them.
//
// The type exists so the AirLabs and FlightAware clients, the enricher, the
// store, and the server all agree on one shape regardless of which provider
// answered. Route lookups are keyed by callsign rather than ICAO24 because the
// same airframe flies different routes, and callsigns arrive from the feed
// space-padded, so callers trim before looking up.
//
// route is a domain package. It imports nothing from this module and holds no
// behaviour, so it is exercised through the packages that consume it rather
// than by its own test suite.
package route
