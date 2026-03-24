// -------------------------------------------------------------------------------
// FlightAware - Response Types
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Defines the response types for the FlightAware AeroAPI. Lookups by flight
// ident return origin and destination airports with ICAO/IATA codes.
// -------------------------------------------------------------------------------

package flightaware

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// flightsResponse wraps the AeroAPI flights endpoint response.
type flightsResponse struct {
	Flights []flight `json:"flights"`
}

// flight represents a single flight from the AeroAPI response.
type flight struct {
	Ident       string  `json:"ident"`
	Origin      airport `json:"origin"`
	Destination airport `json:"destination"`
}

// airport represents an airport in the AeroAPI response.
type airport struct {
	CodeICAO string `json:"code_icao"`
	CodeIATA string `json:"code_iata"`
	Name     string `json:"name"`
}
