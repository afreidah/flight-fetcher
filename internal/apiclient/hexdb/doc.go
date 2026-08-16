// -------------------------------------------------------------------------------
// HexDB - Package Documentation
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Package-level documentation and layering notes for the hexdb package.
// -------------------------------------------------------------------------------

// Package hexdb is the client for the HexDB.io aircraft metadata API. It
// resolves an ICAO24 hex code to registration, type, manufacturer, and operator
// details, and separately resolves an aircraft photo URL.
//
// HexDB requires no authentication, which is why it is the first aircraft
// enrichment source tried: a miss costs nothing but a request, and the OpenSky
// metadata lookup behind it spends credits. The image lookup issues its request
// through DoRaw so it bypasses the shared backoff state - a photo is presented
// as a convenience by the dashboard, and its failure must not delay the
// metadata path that matters.
//
// hexdb sits in the transport layer. It imports apiclient and aircraft, and is
// consumed by the enricher and, for images, by the dashboard server.
package hexdb
