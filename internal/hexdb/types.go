// -------------------------------------------------------------------------------
// HexDB - Response Types
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Defines the response types for the HexDB.io aircraft metadata API. Lookups
// are keyed by ICAO24 hex code and return static aircraft registration data.
// -------------------------------------------------------------------------------

package hexdb

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// AircraftInfo contains metadata about an aircraft looked up by ICAO24 hex
// code from HexDB.io.
type AircraftInfo struct {
	ICAO24           string `json:"icao24"`
	Registration     string `json:"Registration"`
	ManufacturerName string `json:"ManufacturerName"`
	Type             string `json:"Type"`
	OperatorFlagCode string `json:"OperatorFlagCode"`
}

// IsSentinel returns true if the record is a negative cache entry with no
// actual metadata.
func (a *AircraftInfo) IsSentinel() bool {
	return a.Registration == "" && a.ManufacturerName == "" && a.Type == ""
}
