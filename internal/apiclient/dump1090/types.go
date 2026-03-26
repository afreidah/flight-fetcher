// -------------------------------------------------------------------------------
// dump1090 - Response Types
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Defines the full set of fields available from dump1090/readsb/dump1090-fa
// local ADS-B receivers. All fields are optional (pointers) since aircraft
// may not broadcast every field.
// -------------------------------------------------------------------------------

package dump1090

// FeedResponse represents the top-level dump1090 aircraft.json response.
type FeedResponse struct {
	Now      float64    `json:"now"`
	Messages int64      `json:"messages"`
	Aircraft []Aircraft `json:"aircraft"`
}

// Aircraft represents a single aircraft from the dump1090 feed with all
// available ADS-B fields. Fields are pointers because aircraft may not
// broadcast every value.
type Aircraft struct {
	// Core identification
	Hex    string `json:"hex"`    // 24-bit ICAO identifier (6 hex digits)
	Type   string `json:"type"`   // source type: adsb_icao, mlat, tisb, mode_s, etc.
	Flight string `json:"flight"` // callsign or registration (8 chars)

	// Database fields (when receiver has --db-file)
	Registration string `json:"r"`       // aircraft registration from local DB
	AircraftType string `json:"t"`       // aircraft type from local DB
	Description  string `json:"desc"`    // long type name (with --db-file-lt)
	DBFlags      *int   `json:"dbFlags"` // bitfield: 1=military, 2=interesting, 4=PIA, 8=LADD

	// Position
	Lat     *float64 `json:"lat"`
	Lon     *float64 `json:"lon"`
	SeenPos *float64 `json:"seen_pos"` // seconds since last position update

	// Altitude
	AltBaro  *float64 `json:"alt_baro"`  // barometric altitude (feet) or "ground"
	AltGeom  *float64 `json:"alt_geom"`  // geometric (GPS) altitude (feet)
	BaroRate *float64 `json:"baro_rate"` // baro altitude change (ft/min)
	GeomRate *float64 `json:"geom_rate"` // geometric altitude change (ft/min)

	// Speed
	GroundSpeed *float64 `json:"gs"`   // ground speed (knots)
	IAS         *float64 `json:"ias"`  // indicated airspeed (knots)
	TAS         *float64 `json:"tas"`  // true airspeed (knots)
	Mach        *float64 `json:"mach"` // Mach number

	// Direction
	Track       *float64 `json:"track"`       // true track over ground (degrees)
	TrackRate   *float64 `json:"track_rate"`   // track change rate (degrees/sec)
	MagHeading  *float64 `json:"mag_heading"`  // magnetic heading
	TrueHeading *float64 `json:"true_heading"` // true heading
	Roll        *float64 `json:"roll"`         // bank angle (degrees)

	// Navigation & automation
	NavQNH         *float64 `json:"nav_qnh"`          // altimeter setting (hPa)
	NavAltitudeMCP *float64 `json:"nav_altitude_mcp"`  // selected altitude (MCP/FCU)
	NavAltitudeFMS *float64 `json:"nav_altitude_fms"`  // selected altitude (FMS)
	NavHeading     *float64 `json:"nav_heading"`       // selected heading
	NavModes       []string `json:"nav_modes"`         // active: autopilot, vnav, althold, approach, lnav, tcas

	// Transponder
	Squawk    string `json:"squawk"`    // Mode A code (4 octal digits)
	Emergency string `json:"emergency"` // emergency status
	Category  string `json:"category"`  // emitter category (A0-D7)

	// Accuracy & integrity
	NIC     *int     `json:"nic"`      // Navigation Integrity Category
	RC      *float64 `json:"rc"`       // Radius of Containment (meters)
	NICBaro *int     `json:"nic_baro"` // barometric altitude NIC
	NACp    *int     `json:"nac_p"`    // Navigation Accuracy for Position
	NACv    *int     `json:"nac_v"`    // Navigation Accuracy for Velocity
	SIL     *int     `json:"sil"`      // Source Integrity Level
	SILType string   `json:"sil_type"` // SIL interpretation
	GVA     *int     `json:"gva"`      // Geometric Vertical Accuracy
	SDA     *int     `json:"sda"`      // System Design Assurance
	Version *int     `json:"version"`  // ADS-B version (0, 1, 2)

	// Weather (calculated by receiver)
	WindDirection *float64 `json:"wd"`  // wind direction (degrees)
	WindSpeed     *float64 `json:"ws"`  // wind speed (knots)
	OAT           *float64 `json:"oat"` // outside air temperature (°C)
	TAT           *float64 `json:"tat"` // total air temperature (°C)

	// Signal & message data
	Messages int64    `json:"messages"` // total Mode S messages received
	Seen     *float64 `json:"seen"`     // seconds since last message
	RSSI     *float64 `json:"rssi"`     // signal power (dBFS)

	// Status flags
	OnGround bool `json:"ground"` // aircraft is on the ground
	Alert    bool `json:"alert"`  // flight status alert bit
	SPI      bool `json:"spi"`    // special position identification
}

// IsMilitary returns true if the receiver's database flagged this aircraft
// as military (dbFlags bit 0).
func (a *Aircraft) IsMilitary() bool {
	return a.DBFlags != nil && *a.DBFlags&1 != 0
}
