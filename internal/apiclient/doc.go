// -------------------------------------------------------------------------------
// API Client - Package Documentation
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Package-level documentation and layering notes for the apiclient package.
// -------------------------------------------------------------------------------

// Package apiclient provides the HTTP behaviour every external integration in
// this service shares: request construction against a base URL, exponential
// backoff on 429 responses, a response body size limit, JSON decoding, and
// OpenTelemetry spans and metrics per request.
//
// Each provider client embeds a Client rather than reimplementing this. The
// effect is that a new integration inherits timeouts, rate-limit handling, and
// instrumentation by construction, and that backoff state is per-client - one
// provider throttling the service does not stall calls to the others.
//
// The body size limit is a hard bound, not a tuning knob: these are third-party
// endpoints, and an unbounded read of an unexpectedly large or hostile response
// would be an availability problem. DoRaw exists for the narrow case of a call
// that must bypass backoff, such as an image lookup whose failure should not
// slow the enrichment path behind it.
//
// apiclient is the transport layer. It imports nothing from this module;
// provider subpackages embed it and translate wire formats into the aircraft
// and route domain types.
package apiclient
