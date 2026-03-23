// -------------------------------------------------------------------------------
// AirLabs - Response Types
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Defines the response types for the AirLabs flight data API. Lookups are
// keyed by ICAO flight code (callsign) and return route, airport, and aircraft
// information.
// -------------------------------------------------------------------------------

package airlabs

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// FlightRoute contains departure and arrival airport information for a flight.
type FlightRoute struct {
	FlightICAO string `json:"flight_icao"`
	FlightIATA string `json:"flight_iata"`
	DepIATA    string `json:"dep_iata"`
	DepICAO    string `json:"dep_icao"`
	DepName    string `json:"dep_name"`
	ArrIATA    string `json:"arr_iata"`
	ArrICAO    string `json:"arr_icao"`
	ArrName    string `json:"arr_name"`
}
