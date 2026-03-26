// -------------------------------------------------------------------------------
// Aircraft - Domain Types and Classification
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Shared domain type for aircraft metadata used across API clients, the
// enricher, store, and server. Includes classification of aircraft as
// military, law enforcement, or emergency services, plus static lookups
// for aircraft type specifications and airline details. Data sources:
// military hex ranges and aircraft types from tar1090-db, airline data
// from OpenFlights/airline-codes.
// -------------------------------------------------------------------------------

package aircraft

import (
	_ "embed"
	"encoding/json"
	"log"
	"sort"
	"strconv"
	"strings"
)

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
	ICAOTypeCode     string `json:"icao_type_code,omitempty"`
	RegisteredOwners string `json:"registered_owners,omitempty"`
	ImageURL         string `json:"image_url,omitempty"`
}

// IsSentinel returns true if the record is a negative cache entry with no
// actual metadata.
func (a *Info) IsSentinel() bool {
	return a.Registration == "" && a.ManufacturerName == "" && a.Type == ""
}

// -------------------------------------------------------------------------
// CLASSIFICATION
// -------------------------------------------------------------------------

// Classification constants returned by Classify.
const (
	ClassMilitary       = "military"
	ClassLawEnforcement = "law_enforcement"
	ClassEmergency      = "emergency"
)

// Classify returns the classification for an aircraft based on its ICAO24
// hex address and registered owner. Returns empty string for civilian aircraft.
// Priority: military (hex range) > law enforcement > emergency services.
func Classify(icao24, registeredOwners string) string {
	if isMilitaryHex(icao24) {
		return ClassMilitary
	}
	if registeredOwners == "" {
		return ""
	}
	lower := strings.ToLower(registeredOwners)
	for _, kw := range leKeywords {
		if strings.Contains(lower, kw) {
			return ClassLawEnforcement
		}
	}
	for _, kw := range emsKeywords {
		if strings.Contains(lower, kw) {
			return ClassEmergency
		}
	}
	return ""
}

// IsMilitary returns true if the ICAO24 hex address falls within a known
// military allocation range.
func IsMilitary(icao24 string) bool {
	return isMilitaryHex(icao24)
}

// -------------------------------------------------------------------------
// TYPE SPECIFICATIONS
// -------------------------------------------------------------------------

//go:embed aircraft_types.json
var aircraftTypesJSON []byte

// TypeSpec contains ICAO aircraft type classification.
type TypeSpec struct {
	Description string // ICAO description code (e.g., "L2J" = Land, 2 engines, Jet)
	WTC         string // Wake turbulence category: L(ight), M(edium), H(eavy), J (super)
}

var aircraftTypes map[string]TypeSpec

// LookupType returns the type specification for an ICAO type designator
// (e.g., "B738", "A320"). Returns nil if not found.
func LookupType(icaoTypeCode string) *TypeSpec {
	ts, ok := aircraftTypes[strings.ToUpper(icaoTypeCode)]
	if !ok {
		return nil
	}
	return &ts
}

// DescribeAircraftClass returns a human-readable description of an ICAO
// aircraft description code (e.g., "L2J" → "Land, 2 engines, Jet").
func DescribeAircraftClass(desc string) string {
	if len(desc) < 3 {
		return desc
	}
	var parts []string
	switch desc[0] {
	case 'L':
		parts = append(parts, "Land")
	case 'S':
		parts = append(parts, "Sea")
	case 'A':
		parts = append(parts, "Amphibian")
	case 'H':
		return "Helicopter"
	case 'G':
		return "Gyrocopter"
	case 'T':
		return "Tiltrotor"
	}
	parts = append(parts, string(desc[1])+" engine(s)")
	switch desc[2] {
	case 'J':
		parts = append(parts, "Jet")
	case 'T':
		parts = append(parts, "Turboprop")
	case 'P':
		parts = append(parts, "Piston")
	case 'E':
		parts = append(parts, "Electric")
	}
	return strings.Join(parts, ", ")
}

// -------------------------------------------------------------------------
// AIRLINE DETAILS
// -------------------------------------------------------------------------

//go:embed airlines.json
var airlinesJSON []byte

// AirlineInfo contains details about an airline operator.
type AirlineInfo struct {
	Name     string `json:"name"`
	Country  string `json:"country"`
	IATA     string `json:"iata"`
	Callsign string `json:"callsign"`
}

var airlines map[string]AirlineInfo

// LookupAirline returns airline details for an ICAO operator code
// (e.g., "UAL", "DAL"). Returns nil if not found.
func LookupAirline(operatorCode string) *AirlineInfo {
	a, ok := airlines[strings.ToUpper(operatorCode)]
	if !ok {
		return nil
	}
	return &a
}

// -------------------------------------------------------------------------
// MILITARY HEX RANGES
// -------------------------------------------------------------------------

//go:embed military_ranges.json
var militaryRangesJSON []byte

// hexRange is a pair of ICAO24 address bounds (inclusive).
type hexRange struct {
	low  uint32
	high uint32
}

// militaryRanges is a sorted slice of military hex ranges, parsed once
// at init time for efficient binary search.
var militaryRanges []hexRange

func init() {
	var raw struct {
		Military [][2]string `json:"military"`
	}
	if err := json.Unmarshal(militaryRangesJSON, &raw); err != nil {
		log.Fatalf("parsing military ranges: %v", err)
	}
	militaryRanges = make([]hexRange, len(raw.Military))
	for i, pair := range raw.Military {
		low, err := strconv.ParseUint(pair[0], 16, 32)
		if err != nil {
			log.Fatalf("parsing military range low %q: %v", pair[0], err)
		}
		high, err := strconv.ParseUint(pair[1], 16, 32)
		if err != nil {
			log.Fatalf("parsing military range high %q: %v", pair[1], err)
		}
		militaryRanges[i] = hexRange{low: uint32(low), high: uint32(high)}
	}
	sort.Slice(militaryRanges, func(i, j int) bool {
		return militaryRanges[i].low < militaryRanges[j].low
	})

	// Aircraft types
	var rawTypes map[string]struct {
		Desc string `json:"desc"`
		WTC  string `json:"wtc"`
	}
	if err := json.Unmarshal(aircraftTypesJSON, &rawTypes); err != nil {
		log.Fatalf("parsing aircraft types: %v", err)
	}
	aircraftTypes = make(map[string]TypeSpec, len(rawTypes))
	for k, v := range rawTypes {
		aircraftTypes[k] = TypeSpec{Description: v.Desc, WTC: v.WTC}
	}

	// Airlines
	if err := json.Unmarshal(airlinesJSON, &airlines); err != nil {
		log.Fatalf("parsing airlines: %v", err)
	}
}

// isMilitaryHex checks the ICAO24 against military hex ranges.
func isMilitaryHex(icao24 string) bool {
	addr, err := strconv.ParseUint(strings.ToLower(icao24), 16, 32)
	if err != nil {
		return false
	}
	v := uint32(addr)
	i := sort.Search(len(militaryRanges), func(i int) bool {
		return militaryRanges[i].low > v
	}) - 1
	if i < 0 {
		return false
	}
	return v <= militaryRanges[i].high
}

// -------------------------------------------------------------------------
// KEYWORD LISTS
// -------------------------------------------------------------------------

var leKeywords = []string{
	"police", "sheriff", "highway patrol", "law enforcement",
	"customs and border", "border patrol", "us marshals",
	"u.s. marshals", "fbi", "dea", "atf",
	"department of homeland", "dept of homeland",
	"department of justice", "dept of justice",
	"state patrol", "public safety",
}

var emsKeywords = []string{
	"air methods", "air ambulance", "life flight", "lifeflight",
	"medevac", "med-trans", "mercy flight", "reach air",
	"cal fire", "fire department", "fire dept",
	"los angeles fire", "lafd",
	"helicopter emergency", "hems",
	"children's hospital", "childrens hospital",
	"mercy air",
}
