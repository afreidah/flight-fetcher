// -------------------------------------------------------------------------------
// Geo - Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests geographic calculations: haversine distance with known city pairs,
// symmetry, and bounding box computation.
// -------------------------------------------------------------------------------

package geo

import (
	"math"
	"testing"
)

// TestHaversineKm_SamePoint verifies that the distance between identical coordinates is zero.
func TestHaversineKm_SamePoint(t *testing.T) {
	c := Coord{Lat: 34.0928, Lon: -118.3287}
	got := HaversineKm(c, c)
	if got != 0 {
		t.Errorf("HaversineKm(same, same) = %f, want 0", got)
	}
}

// TestHaversineKm_KnownDistances verifies haversine against known airport-to-airport distances.
func TestHaversineKm_KnownDistances(t *testing.T) {
	tests := []struct {
		name    string
		a, b    Coord
		wantKm  float64
		epsilon float64
	}{
		{
			name:    "LAX to SFO",
			a:       Coord{Lat: 33.9425, Lon: -118.4081},
			b:       Coord{Lat: 37.6213, Lon: -122.3790},
			wantKm:  543.0,
			epsilon: 5.0,
		},
		{
			name:    "LAX to JFK",
			a:       Coord{Lat: 33.9425, Lon: -118.4081},
			b:       Coord{Lat: 40.6413, Lon: -73.7781},
			wantKm:  3983.0,
			epsilon: 15.0,
		},
		{
			name:    "London to Paris",
			a:       Coord{Lat: 51.5074, Lon: -0.1278},
			b:       Coord{Lat: 48.8566, Lon: 2.3522},
			wantKm:  344.0,
			epsilon: 5.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HaversineKm(tt.a, tt.b)
			if math.Abs(got-tt.wantKm) > tt.epsilon {
				t.Errorf("HaversineKm() = %f, want %f (+/- %f)", got, tt.wantKm, tt.epsilon)
			}
		})
	}
}

// TestHaversineKm_Symmetry verifies that distance is the same in both directions.
func TestHaversineKm_Symmetry(t *testing.T) {
	a := Coord{Lat: 34.0928, Lon: -118.3287}
	b := Coord{Lat: 40.7128, Lon: -74.0060}
	ab := HaversineKm(a, b)
	ba := HaversineKm(b, a)
	if ab != ba {
		t.Errorf("HaversineKm not symmetric: %f != %f", ab, ba)
	}
}

// TestBBoxAround_ContainsCenter verifies that the bounding box contains the center point.
func TestBBoxAround_ContainsCenter(t *testing.T) {
	c := Coord{Lat: 34.0928, Lon: -118.3287}
	bbox := BBoxAround(c, 50.0)
	if c.Lat < bbox.MinLat || c.Lat > bbox.MaxLat {
		t.Errorf("center lat %f not within bbox [%f, %f]", c.Lat, bbox.MinLat, bbox.MaxLat)
	}
	if c.Lon < bbox.MinLon || c.Lon > bbox.MaxLon {
		t.Errorf("center lon %f not within bbox [%f, %f]", c.Lon, bbox.MinLon, bbox.MaxLon)
	}
}

// TestBBoxAround_SymmetricAroundCenter verifies that the bbox is centered on the input coordinate.
func TestBBoxAround_SymmetricAroundCenter(t *testing.T) {
	c := Coord{Lat: 34.0928, Lon: -118.3287}
	bbox := BBoxAround(c, 50.0)
	midLat := (bbox.MaxLat + bbox.MinLat) / 2.0
	midLon := (bbox.MaxLon + bbox.MinLon) / 2.0
	if math.Abs(midLat-c.Lat) > 1e-10 {
		t.Errorf("bbox not centered on lat: mid=%f, center=%f", midLat, c.Lat)
	}
	if math.Abs(midLon-c.Lon) > 1e-10 {
		t.Errorf("bbox not centered on lon: mid=%f, center=%f", midLon, c.Lon)
	}
}

// TestBBoxAround_EdgePointsWithinRadius verifies that bbox edges are at or beyond the radius.
func TestBBoxAround_EdgePointsWithinRadius(t *testing.T) {
	c := Coord{Lat: 34.0928, Lon: -118.3287}
	radius := 50.0
	bbox := BBoxAround(c, radius)
	for _, edge := range []struct {
		name  string
		coord Coord
	}{
		{"north", Coord{Lat: bbox.MaxLat, Lon: c.Lon}},
		{"south", Coord{Lat: bbox.MinLat, Lon: c.Lon}},
		{"east", Coord{Lat: c.Lat, Lon: bbox.MaxLon}},
		{"west", Coord{Lat: c.Lat, Lon: bbox.MinLon}},
	} {
		dist := HaversineKm(c, edge.coord)
		if dist < radius*0.99 {
			t.Errorf("bbox %s edge at %f km, expected >= %f km", edge.name, dist, radius)
		}
	}
}
