// -------------------------------------------------------------------------------
// dump1090 - Local ADS-B Receiver Client
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// HTTP client for dump1090/readsb/dump1090-fa local ADS-B receivers. Fetches
// the full aircraft feed from the receiver's JSON endpoint. Also provides a
// StateVector adapter for compatibility with the poller's FlightSource
// interface. Embeds the shared apiclient for consistent timeouts, backoff,
// tracing, and metrics.
// -------------------------------------------------------------------------------

package dump1090

import (
	"context"
	"strings"

	"github.com/afreidah/flight-fetcher/internal/aircraft"
	"github.com/afreidah/flight-fetcher/internal/apiclient"
	"github.com/afreidah/flight-fetcher/internal/apiclient/opensky"
	"github.com/afreidah/flight-fetcher/internal/geo"
)

// -------------------------------------------------------------------------
// TYPES
// -------------------------------------------------------------------------

// Client fetches aircraft data from a local dump1090-compatible receiver.
type Client struct {
	*apiclient.Client
}

// -------------------------------------------------------------------------
// PUBLIC API
// -------------------------------------------------------------------------

// NewClient creates a dump1090 client pointed at the given base URL
// (e.g., "http://piaware:8080").
func NewClient(baseURL string) *Client {
	return &Client{
		Client: apiclient.New(apiclient.Options{
			Name:    "dump1090",
			BaseURL: strings.TrimRight(baseURL, "/"),
		}),
	}
}

// Fetch retrieves the full aircraft feed from the receiver. Returns all
// aircraft with all available ADS-B fields.
func (c *Client) Fetch(ctx context.Context) (*FeedResponse, error) {
	feed, err := apiclient.Lookup[FeedResponse](c.Client, ctx, "/data/aircraft.json")
	if err != nil {
		return nil, err
	}
	return feed, nil
}

// GetStates fetches aircraft from the local receiver and returns them as
// OpenSky-compatible StateVectors, filtered to the given bounding box.
// Satisfies the poller.FlightSource interface.
func (c *Client) GetStates(ctx context.Context, bbox geo.BBox) (*opensky.StatesResponse, error) {
	feed, err := c.Fetch(ctx)
	if err != nil {
		return nil, err
	}

	result := &opensky.StatesResponse{
		Time:   int64(feed.Now),
		States: make([]opensky.StateVector, 0, len(feed.Aircraft)),
	}

	for i := range feed.Aircraft {
		sv := feed.Aircraft[i].ToStateVector()
		if sv == nil {
			continue
		}
		if sv.Latitude < bbox.MinLat || sv.Latitude > bbox.MaxLat ||
			sv.Longitude < bbox.MinLon || sv.Longitude > bbox.MaxLon {
			continue
		}
		result.States = append(result.States, *sv)
	}

	return result, nil
}

// ToStateVector converts a dump1090 Aircraft to an OpenSky-compatible
// StateVector. Returns nil if the aircraft has no position data.
func (a *Aircraft) ToStateVector() *opensky.StateVector {
	if a.Lat == nil || a.Lon == nil {
		return nil
	}

	sv := &opensky.StateVector{
		ICAO24:        strings.ToLower(a.Hex),
		Callsign:      strings.TrimSpace(a.Flight),
		OriginCountry: aircraft.CountryFromICAO24(a.Hex),
		Latitude:      *a.Lat,
		Longitude:     *a.Lon,
		OnGround:      a.OnGround,
		Squawk:        a.Squawk,

		Category:     a.Category,
		Emergency:    a.Emergency,
		NavModes:     a.NavModes,
		Registration: strings.TrimSpace(a.Registration),
		AircraftType: strings.TrimSpace(a.AircraftType),
		Description:  strings.TrimSpace(a.Description),
	}

	if a.AltBaro != nil {
		sv.BaroAltitude = *a.AltBaro * 0.3048 // feet to meters
	}
	if a.GroundSpeed != nil {
		sv.Velocity = *a.GroundSpeed * 0.514444 // knots to m/s
	}
	if a.Track != nil {
		sv.Heading = *a.Track
	}
	if a.BaroRate != nil {
		sv.VerticalRate = *a.BaroRate * 0.00508 // ft/min to m/s
	}
	if a.AltGeom != nil {
		v := *a.AltGeom * 0.3048
		sv.GeoAltitude = &v
	}
	if a.NavAltitudeMCP != nil {
		v := *a.NavAltitudeMCP * 0.3048
		sv.NavAltitudeMCP = &v
	}
	if a.NavHeading != nil {
		v := *a.NavHeading
		sv.NavHeading = &v
	}
	if a.Seen != nil {
		v := *a.Seen
		sv.SeenSec = &v
	}
	if a.RSSI != nil {
		v := *a.RSSI
		sv.RSSI = &v
	}
	if a.Messages != 0 {
		v := a.Messages
		sv.MessageCount = &v
	}
	if a.DBFlags != nil {
		mil := a.IsMilitary()
		sv.IsMilitary = &mil
	}

	return sv
}
