// -------------------------------------------------------------------------------
// FlightAware - Package Documentation
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Package-level documentation and layering notes for the flightaware package.
// -------------------------------------------------------------------------------

// Package flightaware is the client for the FlightAware AeroAPI. It resolves a
// callsign, which AeroAPI calls an ident, to a route.Info giving the departure
// and arrival airports.
//
// FlightAware is the fallback route source, tried after AirLabs. Ordering the
// two this way is a cost decision rather than a quality one: AeroAPI bills per
// request, so it is only reached for callsigns the cheaper source could not
// resolve. Either source alone is sufficient, and configuring neither disables
// route enrichment.
//
// flightaware sits in the transport layer. It imports apiclient and route, and
// is consumed by the enricher.
package flightaware
