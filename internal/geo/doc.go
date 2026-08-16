// -------------------------------------------------------------------------------
// Geo - Package Documentation
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Package-level documentation and layering notes for the geo package.
// -------------------------------------------------------------------------------

// Package geo provides the coordinate types and spherical-distance maths the
// rest of the service filters on: a Coord, a BBox, the bounding box that
// circumscribes a radius around a point, and haversine distance between two
// points.
//
// The split between BBoxAround and HaversineKm is deliberate. Flight APIs
// accept a rectangular bounding box, so a radius query is issued as the box
// that circumscribes it and the corners are then trimmed by an exact haversine
// check on the results. Callers that skip the second step will see aircraft
// outside their configured radius.
//
// geo is a leaf package. It imports nothing from this module and everything
// from apiclient upward may import it.
package geo
