package hexdb

// AircraftInfo contains metadata about an aircraft looked up by ICAO24 hex code.
type AircraftInfo struct {
	ICAO24         string `json:"icao24"`
	Registration   string `json:"Registration"`
	ManufacturerName string `json:"ManufacturerName"`
	Type           string `json:"Type"`
	OperatorFlagCode string `json:"OperatorFlagCode"`
}
