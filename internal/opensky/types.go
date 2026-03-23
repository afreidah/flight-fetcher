package opensky

// StateVector represents a single aircraft state from the OpenSky API.
type StateVector struct {
	ICAO24       string
	Callsign     string
	OriginCountry string
	Longitude    float64
	Latitude     float64
	BaroAltitude float64
	Velocity     float64
	Heading      float64
	VerticalRate float64
	OnGround     bool
}

// StatesResponse is the top-level response from GET /states/all.
type StatesResponse struct {
	Time   int64
	States []StateVector
}
