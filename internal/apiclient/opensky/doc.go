// -------------------------------------------------------------------------------
// OpenSky - Package Documentation
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Package-level documentation and layering notes for the opensky package.
// -------------------------------------------------------------------------------

// Package opensky is the client for the OpenSky Network REST API, the service's
// primary wide-area flight source. It supplies state vectors for a bounding box
// and aircraft metadata for an ICAO24, authenticating with the OAuth2 client
// credentials flow and caching the access token until it expires.
//
// StateVector is defined here rather than in a domain package because it is the
// shape the rest of the pipeline settled on for live flight state; the dump1090
// client adapts its own feed into the same type so a poller cannot tell the two
// sources apart.
//
// OpenSky enforces a credit budget per account, which is why polling interval
// and radius are configurable and why backoff is inherited from apiclient. A
// client instance owns its token cache and backoff state, so separate instances
// are the way to keep one workload's rate-limit troubles away from another's.
//
// opensky sits in the transport layer. It imports apiclient, aircraft, and geo,
// and is consumed by the poller, the squawk monitor, and the enricher.
package opensky
