// -------------------------------------------------------------------------------
// Route - Flight Route Domain Types
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Shared domain type for flight route information used across API clients
// (AirLabs, FlightAware), the enricher, store, and server.
// -------------------------------------------------------------------------------

package route

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Info contains departure and arrival airport information for a flight.
type Info struct {
	FlightICAO string `json:"flight_icao"`
	FlightIATA string `json:"flight_iata"`
	DepIATA    string `json:"dep_iata"`
	DepICAO    string `json:"dep_icao"`
	DepName    string `json:"dep_name"`
	ArrIATA    string `json:"arr_iata"`
	ArrICAO    string `json:"arr_icao"`
	ArrName    string `json:"arr_name"`
}
