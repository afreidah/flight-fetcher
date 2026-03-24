// -------------------------------------------------------------------------------
// Squawk - Domain Types
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Defines the domain types for squawk alert data, decoupled from the sqlc
// generated store types.
// -------------------------------------------------------------------------------

package squawk

import "time"

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Alert represents a detected emergency squawk event.
type Alert struct {
	ID       int64     `json:"id"`
	ICAO24   string    `json:"icao24"`
	Callsign string    `json:"callsign"`
	Squawk   string    `json:"squawk"`
	Lat      float64   `json:"lat"`
	Lon      float64   `json:"lon"`
	SeenAt   time.Time `json:"seen_at"`
}
