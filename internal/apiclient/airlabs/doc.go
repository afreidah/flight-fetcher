// -------------------------------------------------------------------------------
// AirLabs - Package Documentation
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Package-level documentation and layering notes for the airlabs package.
// -------------------------------------------------------------------------------

// Package airlabs is the client for the AirLabs flight data API. It resolves a
// callsign to a route.Info giving the departure and arrival airports.
//
// AirLabs is the first route enrichment source when an API key is configured,
// with FlightAware behind it. Route lookups are keyed by callsign, which the
// feed delivers space-padded, so callers trim before looking up. A callsign
// that resolves to nothing is a normal outcome rather than an error: general
// aviation traffic has no scheduled route to find.
//
// The API key is a paid credential with a request quota, which is why route
// enrichment is optional and is skipped entirely when no key is present.
//
// airlabs sits in the transport layer. It imports apiclient and route, and is
// consumed by the enricher.
package airlabs
