// -------------------------------------------------------------------------------
// Aircraft - Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests the aircraft domain type: sentinel detection, military hex range
// lookup, and owner-based classification for law enforcement and emergency
// services.
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
		{
			name: "new fields only does not affect sentinel",
			info: Info{ICAO24: "abc123", ICAOTypeCode: "B738", RegisteredOwners: "United Airlines"},
			want: true,
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

func TestIsMilitary(t *testing.T) {
	tests := []struct {
		name   string
		icao24 string
		want   bool
	}{
		{name: "US military low bound", icao24: "adf7c8", want: true},
		{name: "US military mid", icao24: "ae1234", want: true},
		{name: "US military high bound", icao24: "afffff", want: true},
		{name: "US civilian below military", icao24: "adf7c7", want: false},
		{name: "US civilian above military", icao24: "b00000", want: false},
		{name: "UK military", icao24: "43c500", want: true},
		{name: "Canada military", icao24: "c30000", want: true},
		{name: "civilian N-reg", icao24: "a835af", want: false},
		{name: "empty string", icao24: "", want: false},
		{name: "invalid hex", icao24: "zzzzzz", want: false},
		{name: "uppercase", icao24: "AE1234", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsMilitary(tt.icao24); got != tt.want {
				t.Errorf("IsMilitary(%q) = %v, want %v", tt.icao24, got, tt.want)
			}
		})
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		name   string
		icao24 string
		owner  string
		want   string
	}{
		// Military wins over everything
		{name: "military hex", icao24: "ae1234", owner: "", want: ClassMilitary},
		{name: "military hex with owner", icao24: "ae1234", owner: "US Air Force", want: ClassMilitary},

		// Law enforcement
		{name: "police", icao24: "a12345", owner: "Los Angeles Police Department", want: ClassLawEnforcement},
		{name: "sheriff", icao24: "a12345", owner: "San Bernardino County Sheriff", want: ClassLawEnforcement},
		{name: "highway patrol", icao24: "a12345", owner: "California Highway Patrol", want: ClassLawEnforcement},
		{name: "CBP", icao24: "a12345", owner: "Customs and Border Protection", want: ClassLawEnforcement},
		{name: "DHS", icao24: "a12345", owner: "Department of Homeland Security", want: ClassLawEnforcement},

		// Emergency services
		{name: "air methods", icao24: "a12345", owner: "Air Methods Corporation", want: ClassEmergency},
		{name: "cal fire", icao24: "a12345", owner: "Cal Fire", want: ClassEmergency},
		{name: "life flight", icao24: "a12345", owner: "Life Flight Network", want: ClassEmergency},
		{name: "fire dept", icao24: "a12345", owner: "Los Angeles Fire Department", want: ClassEmergency},
		{name: "mercy air", icao24: "a12345", owner: "Mercy Air Service", want: ClassEmergency},

		// Civilian
		{name: "airline", icao24: "a12345", owner: "United Airlines", want: ""},
		{name: "private", icao24: "a12345", owner: "John Smith", want: ""},
		{name: "empty owner", icao24: "a12345", owner: "", want: ""},

		// Case insensitive
		{name: "case insensitive", icao24: "a12345", owner: "POLICE DEPARTMENT", want: ClassLawEnforcement},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.icao24, tt.owner); got != tt.want {
				t.Errorf("Classify(%q, %q) = %q, want %q", tt.icao24, tt.owner, got, tt.want)
			}
		})
	}
}

func TestLookupType(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		wantNil  bool
		wantDesc string
		wantWTC  string
	}{
		{name: "B738", code: "B738", wantDesc: "L2J", wantWTC: "M"},
		{name: "C172", code: "C172", wantDesc: "L1P", wantWTC: "L"},
		{name: "B77W heavy", code: "B77W", wantDesc: "L2J", wantWTC: "H"},
		{name: "case insensitive", code: "b738", wantDesc: "L2J", wantWTC: "M"},
		{name: "not found", code: "XX99", wantNil: true},
		{name: "empty", code: "", wantNil: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LookupType(tt.code)
			if tt.wantNil {
				if got != nil {
					t.Errorf("LookupType(%q) = %+v, want nil", tt.code, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("LookupType(%q) = nil, want result", tt.code)
			}
			if got.Description != tt.wantDesc {
				t.Errorf("Description = %q, want %q", got.Description, tt.wantDesc)
			}
			if got.WTC != tt.wantWTC {
				t.Errorf("WTC = %q, want %q", got.WTC, tt.wantWTC)
			}
		})
	}
}

func TestDescribeAircraftClass(t *testing.T) {
	tests := []struct {
		desc string
		want string
	}{
		{"L2J", "Land, 2 engine(s), Jet"},
		{"L1P", "Land, 1 engine(s), Piston"},
		{"L4T", "Land, 4 engine(s), Turboprop"},
		{"S2J", "Sea, 2 engine(s), Jet"},
		{"A1P", "Amphibian, 1 engine(s), Piston"},
		{"H1T", "Helicopter"},
		{"G1P", "Gyrocopter"},
		{"T2T", "Tiltrotor"},
		{"L1E", "Land, 1 engine(s), Electric"},
		{"", ""},
		{"AB", "AB"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := DescribeAircraftClass(tt.desc)
			if got != tt.want {
				t.Errorf("DescribeAircraftClass(%q) = %q, want %q", tt.desc, got, tt.want)
			}
		})
	}
}

func TestLookupAirline(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		wantNil  bool
		wantName string
	}{
		{name: "United", code: "UAL", wantName: "United Airlines"},
		{name: "Delta", code: "DAL", wantName: "Delta Air Lines"},
		{name: "Southwest", code: "SWA", wantName: "Southwest Airlines"},
		{name: "case insensitive", code: "ual", wantName: "United Airlines"},
		{name: "not found", code: "XX99", wantNil: true},
		{name: "empty", code: "", wantNil: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LookupAirline(tt.code)
			if tt.wantNil {
				if got != nil {
					t.Errorf("LookupAirline(%q) = %+v, want nil", tt.code, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("LookupAirline(%q) = nil, want result", tt.code)
			}
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
		})
	}
}
