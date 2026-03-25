// -------------------------------------------------------------------------------
// Aircraft - Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests the aircraft domain type sentinel detection.
// -------------------------------------------------------------------------------

package aircraft

import "testing"

// TestIsSentinel verifies sentinel detection for negative cache entries.
func TestIsSentinel(t *testing.T) {
	tests := []struct {
		name string
		info Info
		want bool
	}{
		{
			name: "empty fields is sentinel",
			info: Info{ICAO24: "abc123"},
			want: true,
		},
		{
			name: "with registration is not sentinel",
			info: Info{ICAO24: "abc123", Registration: "N12345"},
			want: false,
		},
		{
			name: "with type is not sentinel",
			info: Info{ICAO24: "abc123", Type: "737-800"},
			want: false,
		},
		{
			name: "with manufacturer is not sentinel",
			info: Info{ICAO24: "abc123", ManufacturerName: "Boeing"},
			want: false,
		},
		{
			name: "fully populated is not sentinel",
			info: Info{ICAO24: "abc123", Registration: "N12345", ManufacturerName: "Boeing", Type: "737-800", OperatorFlagCode: "UAL"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.info.IsSentinel(); got != tt.want {
				t.Errorf("IsSentinel() = %v, want %v", got, tt.want)
			}
		})
	}
}
