// -------------------------------------------------------------------------------
// Aircraft - ICAO24 Country Lookup
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Maps a 24-bit ICAO aircraft address to its registering country. ICAO Annex
// 10 Volume III allocates hex prefixes to national authorities; we embed the
// ranges for the registries that account for the vast majority of observed
// traffic. Sources that already carry OriginCountry (OpenSky) bypass this
// lookup; local sources like dump1090 call into it because the antenna feed
// never includes country.
//
// This table is deliberately non-exhaustive — it covers the top ~25 registries
// that cover >95% of airliner and significant GA traffic worldwide. Unknown
// hex addresses return an empty string so the UI falls back to "unknown".
// Extend from ICAO Annex 10 Vol III Appendix as needed.
// -------------------------------------------------------------------------------

package aircraft

import (
	"sort"
	"strconv"
	"strings"
)

// countryRange describes a contiguous ICAO24 hex block allocated to a
// registering state. Ranges are inclusive on both ends.
type countryRange struct {
	start   uint32
	end     uint32
	country string
}

// countryRanges is sorted by start ascending so CountryFromICAO24 can
// binary-search. Ranges do not overlap.
var countryRanges = []countryRange{
	{0x0D0000, 0x0D7FFF, "Mexico"},
	{0x100000, 0x1FFFFF, "Russia"},
	{0x300000, 0x33FFFF, "Italy"},
	{0x340000, 0x37FFFF, "Spain"},
	{0x380000, 0x3BFFFF, "France"},
	{0x3C0000, 0x3FFFFF, "Germany"},
	{0x400000, 0x43FFFF, "United Kingdom"},
	{0x440000, 0x447FFF, "Austria"},
	{0x448000, 0x44FFFF, "Belgium"},
	{0x458000, 0x45FFFF, "Denmark"},
	{0x460000, 0x467FFF, "Finland"},
	{0x478000, 0x47FFFF, "Norway"},
	{0x480000, 0x487FFF, "Netherlands"},
	{0x4A8000, 0x4AFFFF, "Sweden"},
	{0x4B0000, 0x4B7FFF, "Switzerland"},
	{0x4B8000, 0x4BFFFF, "Turkey"},
	{0x710000, 0x717FFF, "South Korea"},
	{0x780000, 0x7BFFFF, "China"},
	{0x7C0000, 0x7FFFFF, "Australia"},
	{0x800000, 0x83FFFF, "India"},
	{0x840000, 0x87FFFF, "Japan"},
	{0x896000, 0x896FFF, "United Arab Emirates"},
	{0xA00000, 0xAFFFFF, "United States"},
	{0xC00000, 0xC3FFFF, "Canada"},
	{0xE00000, 0xE3FFFF, "Argentina"},
	{0xE40000, 0xE7FFFF, "Brazil"},
}

// CountryFromICAO24 returns the registering country for the given 24-bit
// ICAO address (hex, case-insensitive, with or without whitespace). Returns
// the empty string if the address is unparseable or the range is not in
// the local table.
func CountryFromICAO24(hex string) string {
	s := strings.TrimSpace(hex)
	if s == "" {
		return ""
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return ""
	}
	addr := uint32(v)

	i := sort.Search(len(countryRanges), func(i int) bool {
		return countryRanges[i].end >= addr
	})
	if i == len(countryRanges) {
		return ""
	}
	r := countryRanges[i]
	if addr < r.start {
		return ""
	}
	return r.country
}
