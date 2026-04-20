// -------------------------------------------------------------------------------
// Aircraft - Country Lookup Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
// -------------------------------------------------------------------------------

package aircraft

import "testing"

// TestCountryFromICAO24 walks a range of hex addresses covering every branch
// of the lookup: start-of-block, mid-block, end-of-block, gaps between
// allocated ranges, below the lowest range, and malformed input.
func TestCountryFromICAO24(t *testing.T) {
	tests := []struct {
		name string
		hex  string
		want string
	}{
		{name: "US start", hex: "a00000", want: "United States"},
		{name: "US mid", hex: "a1ed21", want: "United States"},
		{name: "US end", hex: "afffff", want: "United States"},
		{name: "Canada start", hex: "c00000", want: "Canada"},
		{name: "Canada mid", hex: "c20abc", want: "Canada"},
		{name: "UK", hex: "43ffff", want: "United Kingdom"},
		{name: "Germany", hex: "3cabcd", want: "Germany"},
		{name: "France", hex: "39dead", want: "France"},
		{name: "Japan", hex: "850001", want: "Japan"},
		{name: "China", hex: "780000", want: "China"},
		{name: "Australia", hex: "7cdead", want: "Australia"},
		{name: "Brazil", hex: "e5beef", want: "Brazil"},
		{name: "Mexico", hex: "0d1234", want: "Mexico"},
		{name: "case insensitive", hex: "A1ED21", want: "United States"},
		{name: "with whitespace", hex: " a1ed21 ", want: "United States"},
		{name: "below lowest range", hex: "000001", want: ""},
		{name: "gap between ranges (post-UAE, pre-US)", hex: "900000", want: ""},
		{name: "unparseable", hex: "zzzzzz", want: ""},
		{name: "empty", hex: "", want: ""},
		{name: "whitespace only", hex: "   ", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CountryFromICAO24(tt.hex); got != tt.want {
				t.Errorf("CountryFromICAO24(%q) = %q, want %q", tt.hex, got, tt.want)
			}
		})
	}
}
