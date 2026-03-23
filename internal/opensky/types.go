// -------------------------------------------------------------------------------
// OpenSky - Response Types
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Defines the parsed response types for the OpenSky Network REST API. The raw
// API returns state vectors as heterogeneous arrays; these types represent the
// parsed, typed form used throughout the application.
// -------------------------------------------------------------------------------

package opensky

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// StateVector represents a single aircraft state from the OpenSky API.
type StateVector struct {
	ICAO24        string  `json:"icao24"`
	Callsign      string  `json:"callsign"`
	OriginCountry string  `json:"origin_country"`
	Longitude     float64 `json:"longitude"`
	Latitude      float64 `json:"latitude"`
	BaroAltitude  float64 `json:"baro_altitude"`
	Velocity      float64 `json:"velocity"`
	Heading       float64 `json:"heading"`
	VerticalRate  float64 `json:"vertical_rate"`
	OnGround      bool    `json:"on_ground"`
}

// StatesResponse is the top-level response from GET /states/all.
type StatesResponse struct {
	Time   int64
	States []StateVector
}
