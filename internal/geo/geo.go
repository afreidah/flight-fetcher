package geo

import "math"

const earthRadiusKm = 6371.0

// Coord represents a geographic coordinate.
type Coord struct {
	Lat float64
	Lon float64
}

// BBox represents a geographic bounding box.
type BBox struct {
	MinLat float64
	MaxLat float64
	MinLon float64
	MaxLon float64
}

// BBoxAround returns a bounding box circumscribing a circle of the given radius (km) around c.
func BBoxAround(c Coord, radiusKm float64) BBox {
	latDelta := radiusKm / earthRadiusKm * (180.0 / math.Pi)
	lonDelta := latDelta / math.Cos(c.Lat*math.Pi/180.0)
	return BBox{
		MinLat: c.Lat - latDelta,
		MaxLat: c.Lat + latDelta,
		MinLon: c.Lon - lonDelta,
		MaxLon: c.Lon + lonDelta,
	}
}

// HaversineKm returns the great-circle distance in km between two coordinates.
func HaversineKm(a, b Coord) float64 {
	dLat := (b.Lat - a.Lat) * math.Pi / 180.0
	dLon := (b.Lon - a.Lon) * math.Pi / 180.0
	aLat := a.Lat * math.Pi / 180.0
	bLat := b.Lat * math.Pi / 180.0

	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(aLat)*math.Cos(bLat)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusKm * math.Asin(math.Sqrt(h))
}
