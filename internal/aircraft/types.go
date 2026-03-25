// -------------------------------------------------------------------------------
// Aircraft - Domain Types
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Shared domain type for aircraft metadata used across API clients (HexDB),
// the enricher, store, and server.
// -------------------------------------------------------------------------------

package aircraft

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Info contains metadata about an aircraft identified by ICAO24 hex code.
type Info struct {
	ICAO24           string `json:"icao24"`
	Registration     string `json:"registration"`
	ManufacturerName string `json:"manufacturer_name"`
	Type             string `json:"type"`
	OperatorFlagCode string `json:"operator_flag_code"`
}

// IsSentinel returns true if the record is a negative cache entry with no
// actual metadata.
func (a *Info) IsSentinel() bool {
	return a.Registration == "" && a.ManufacturerName == "" && a.Type == ""
}
