// -------------------------------------------------------------------------------
// HexDB - Response Types
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Maps HexDB.io API responses to the shared aircraft domain type.
// -------------------------------------------------------------------------------

package hexdb

import "github.com/afreidah/flight-fetcher/internal/aircraft"

// AircraftInfo is an alias for the shared domain type, maintained for
// backward compatibility with the HexDB client's Lookup return type.
type AircraftInfo = aircraft.Info
