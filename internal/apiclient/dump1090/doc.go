// -------------------------------------------------------------------------------
// dump1090 - Package Documentation
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Package-level documentation and layering notes for the dump1090 package.
// -------------------------------------------------------------------------------

// Package dump1090 is the client for a local ADS-B receiver running dump1090,
// dump1090-fa, or readsb. It fetches the receiver's full aircraft feed and
// adapts it into opensky.StateVector values.
//
// That adaptation is the point of the package. A locally received aircraft and
// one reported by OpenSky reach the poller as the same type, so the antenna is
// just another flight source rather than a parallel code path. The receiver's
// JSON carries considerably more per-aircraft detail than the poller consumes;
// the surplus fields are decoded and kept so later work can use them without
// revisiting the wire format.
//
// A local receiver has no credit budget and no authentication, and it reports
// only what its own antenna hears, so it is normally polled far more frequently
// than OpenSky and covers a smaller area. Running both sources against a shared
// dedup state is what lets the service prefer whichever one is hearing a given
// aircraft.
//
// dump1090 sits in the transport layer. It imports apiclient, opensky for the
// shared state vector type, aircraft, and geo.
package dump1090
