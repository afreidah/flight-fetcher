// -------------------------------------------------------------------------------
// DedupState - Shared Poller Deduplication
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Holds the in-memory dedup maps that prevent duplicate enrichment requests
// and redundant sighting log writes. Designed to be shared across multiple
// Poller instances (e.g. one per flight source) so that when two sources
// observe the same aircraft, only the first observation triggers enrichment.
// -------------------------------------------------------------------------------

package poller

import (
	"sync"
	"time"
)

// lastPos tracks the last logged position for sighting deduplication.
type lastPos struct {
	lat float64
	lon float64
}

// DedupState is the shared state that lets concurrent pollers avoid
// double-enriching and double-logging sightings for the same aircraft.
type DedupState struct {
	mu            sync.Mutex
	evictInterval time.Duration

	seenICAO   map[string]bool
	seenRoutes map[string]bool
	lastSight  map[string]lastPos
	lastEvict  time.Time
}

// NewDedupState returns a DedupState that evicts its caches every
// evictInterval. The interval is advisory: eviction is checked lazily
// at the start of each poll cycle via MaybeEvict.
func NewDedupState(evictInterval time.Duration) *DedupState {
	return &DedupState{
		evictInterval: evictInterval,
		seenICAO:      make(map[string]bool),
		seenRoutes:    make(map[string]bool),
		lastSight:     make(map[string]lastPos),
		lastEvict:     time.Now(),
	}
}

// MaybeEvict clears the caches if the eviction interval has elapsed since
// the last eviction. Returns (icaoCount, routeCount) of the state that was
// discarded, or (0, 0) if no eviction occurred. Safe to call from multiple
// pollers; only the first caller in a window does the work.
func (d *DedupState) MaybeEvict() (icaoCount, routeCount int, evicted bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if time.Since(d.lastEvict) < d.evictInterval {
		return 0, 0, false
	}

	icaoCount = len(d.seenICAO)
	routeCount = len(d.seenRoutes)
	d.seenICAO = make(map[string]bool)
	d.seenRoutes = make(map[string]bool)
	d.lastSight = make(map[string]lastPos)
	d.lastEvict = time.Now()
	return icaoCount, routeCount, true
}

// NeedsEnrichment reports whether the ICAO24 or callsign is unknown to the
// dedup state and therefore should be queued for enrichment.
func (d *DedupState) NeedsEnrichment(icao24, callsign string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.seenICAO[icao24] {
		return true
	}
	if callsign != "" && !d.seenRoutes[callsign] {
		return true
	}
	return false
}

// MarkICAOSeen records that the given ICAO24's metadata has been resolved
// (found or confirmed absent) so future pollers skip re-queuing it.
func (d *DedupState) MarkICAOSeen(icao24 string) {
	d.mu.Lock()
	d.seenICAO[icao24] = true
	d.mu.Unlock()
}

// isICAOSeen reports whether the given ICAO24 has been resolved. Used by
// workers to re-check state after dequeue, since another poller's worker
// may have completed the same enrichment while the request was queued.
func (d *DedupState) isICAOSeen(icao24 string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.seenICAO[icao24]
}

// isRouteSeen reports whether the given callsign's route has been resolved.
func (d *DedupState) isRouteSeen(callsign string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.seenRoutes[callsign]
}

// MarkRouteSeen records that the given callsign's route has been resolved
// so future pollers skip re-queuing it.
func (d *DedupState) MarkRouteSeen(callsign string) {
	d.mu.Lock()
	d.seenRoutes[callsign] = true
	d.mu.Unlock()
}

// PositionChanged returns true if the aircraft has moved enough since the
// last logged sighting to warrant a new record. Updates the tracking map.
// The threshold is fixed at ~500 m (sightingMinMove in poller.go).
func (d *DedupState) PositionChanged(icao24 string, lat, lon float64) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	prev, ok := d.lastSight[icao24]
	if !ok {
		d.lastSight[icao24] = lastPos{lat: lat, lon: lon}
		return true
	}

	dlat := lat - prev.lat
	dlon := lon - prev.lon
	if dlat < 0 {
		dlat = -dlat
	}
	if dlon < 0 {
		dlon = -dlon
	}
	if dlat < sightingMinMove && dlon < sightingMinMove {
		return false
	}

	d.lastSight[icao24] = lastPos{lat: lat, lon: lon}
	return true
}
