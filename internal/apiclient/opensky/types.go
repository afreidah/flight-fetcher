// -------------------------------------------------------------------------------
// OpenSky - Response Types
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Defines the parsed response types for the OpenSky Network REST API. The raw
// API returns state vectors as heterogeneous JSON arrays; UnmarshalJSON on
// StateVector handles the positional decoding and null-safety.
// -------------------------------------------------------------------------------

package opensky

import (
	"encoding/json"
	"fmt"
)

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
	Squawk        string  `json:"squawk"`
}

// stateVectorMinFields is the minimum number of elements in a raw OpenSky
// state vector array.
const stateVectorMinFields = 15

// UnmarshalJSON decodes a state vector from either the OpenSky API's
// positional JSON array format or a standard JSON object. Null fields in the
// array format decode to zero values.
func (sv *StateVector) UnmarshalJSON(data []byte) error {
	// Standard JSON object (e.g. from our own API or Redis)
	if len(data) > 0 && data[0] == '{' {
		type alias StateVector
		var a alias
		if err := json.Unmarshal(data, &a); err != nil {
			return err
		}
		*sv = StateVector(a)
		return nil
	}

	// OpenSky positional array format
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("decoding state vector array: %w", err)
	}
	if len(raw) < stateVectorMinFields {
		return fmt.Errorf("state vector too short: %d elements, need %d", len(raw), stateVectorMinFields)
	}

	decodeString(raw[0], &sv.ICAO24)
	decodeString(raw[1], &sv.Callsign)
	decodeString(raw[2], &sv.OriginCountry)
	decodeFloat(raw[5], &sv.Longitude)
	decodeFloat(raw[6], &sv.Latitude)
	decodeFloat(raw[7], &sv.BaroAltitude)
	decodeBool(raw[8], &sv.OnGround)
	decodeFloat(raw[9], &sv.Velocity)
	decodeFloat(raw[10], &sv.Heading)
	decodeFloat(raw[11], &sv.VerticalRate)
	decodeString(raw[14], &sv.Squawk)
	return nil
}

// StatesResponse is the top-level response from GET /states/all.
type StatesResponse struct {
	Time   int64         `json:"time"`
	States []StateVector `json:"states"`
}

// -------------------------------------------------------------------------
// INTERNALS
// -------------------------------------------------------------------------

// decodeString unmarshals a JSON value into a string, leaving dst unchanged
// if the value is null or not a string.
func decodeString(raw json.RawMessage, dst *string) {
	var v *string
	if json.Unmarshal(raw, &v) == nil && v != nil {
		*dst = *v
	}
}

// decodeFloat unmarshals a JSON value into a float64, leaving dst unchanged
// if the value is null or not a number.
func decodeFloat(raw json.RawMessage, dst *float64) {
	var v *float64
	if json.Unmarshal(raw, &v) == nil && v != nil {
		*dst = *v
	}
}

// decodeBool unmarshals a JSON value into a bool, leaving dst unchanged
// if the value is null or not a boolean.
func decodeBool(raw json.RawMessage, dst *bool) {
	var v *bool
	if json.Unmarshal(raw, &v) == nil && v != nil {
		*dst = *v
	}
}
