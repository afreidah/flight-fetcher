// -------------------------------------------------------------------------------
// Enricher - Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests aircraft metadata enrichment logic: cache hits, new aircraft lookups,
// missing HexDB data, and error handling for store and lookup failures.
// -------------------------------------------------------------------------------

package enricher

import (
	"context"
	"errors"
	"testing"

	"github.com/afreidah/flight-fetcher/internal/hexdb"

	"go.uber.org/mock/gomock"
)

// TestEnrich_AlreadyCached verifies that a cached aircraft returns false (not new).
func TestEnrich_AlreadyCached(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockAircraftStore(ctrl)
	lookup := NewMockAircraftLookup(ctrl)

	store.EXPECT().
		GetAircraftMeta(gomock.Any(), "abc123").
		Return(&hexdb.AircraftInfo{ICAO24: "abc123"}, nil)

	enr := New(lookup, store)
	got := enr.Enrich(context.Background(), "abc123")
	if got != false {
		t.Errorf("Enrich() = %v, want false for already cached aircraft", got)
	}
}

// TestEnrich_NewAircraft_LookupSuccess verifies that a new aircraft is looked up and saved.
func TestEnrich_NewAircraft_LookupSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockAircraftStore(ctrl)
	lookup := NewMockAircraftLookup(ctrl)

	info := &hexdb.AircraftInfo{
		ICAO24:           "abc123",
		Registration:     "N12345",
		ManufacturerName: "Boeing",
		Type:             "737-800",
		OperatorFlagCode: "UAL",
	}

	store.EXPECT().
		GetAircraftMeta(gomock.Any(), "abc123").
		Return(nil, nil)
	lookup.EXPECT().
		Lookup(gomock.Any(), "abc123").
		Return(info, nil)
	store.EXPECT().
		SaveAircraftMeta(gomock.Any(), info).
		Return(nil)

	enr := New(lookup, store)
	got := enr.Enrich(context.Background(), "abc123")
	if got != true {
		t.Errorf("Enrich() = %v, want true for new aircraft", got)
	}
}

// TestEnrich_NewAircraft_NotInHexDB verifies that a new aircraft not in HexDB still returns true.
func TestEnrich_NewAircraft_NotInHexDB(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockAircraftStore(ctrl)
	lookup := NewMockAircraftLookup(ctrl)

	store.EXPECT().
		GetAircraftMeta(gomock.Any(), "abc123").
		Return(nil, nil)
	lookup.EXPECT().
		Lookup(gomock.Any(), "abc123").
		Return(nil, nil)

	enr := New(lookup, store)
	got := enr.Enrich(context.Background(), "abc123")
	if got != true {
		t.Errorf("Enrich() = %v, want true for new aircraft not in hexdb", got)
	}
}

// TestEnrich_StoreGetError verifies that a store read failure returns false.
func TestEnrich_StoreGetError(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockAircraftStore(ctrl)
	lookup := NewMockAircraftLookup(ctrl)

	store.EXPECT().
		GetAircraftMeta(gomock.Any(), "abc123").
		Return(nil, errors.New("db down"))

	enr := New(lookup, store)
	got := enr.Enrich(context.Background(), "abc123")
	if got != false {
		t.Errorf("Enrich() = %v, want false when store read fails", got)
	}
}

// TestEnrich_LookupError verifies that a lookup failure still returns true (new aircraft).
func TestEnrich_LookupError(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockAircraftStore(ctrl)
	lookup := NewMockAircraftLookup(ctrl)

	store.EXPECT().
		GetAircraftMeta(gomock.Any(), "abc123").
		Return(nil, nil)
	lookup.EXPECT().
		Lookup(gomock.Any(), "abc123").
		Return(nil, errors.New("timeout"))

	enr := New(lookup, store)
	got := enr.Enrich(context.Background(), "abc123")
	if got != true {
		t.Errorf("Enrich() = %v, want true when lookup fails (still a new aircraft)", got)
	}
}

// TestEnrich_SaveError verifies that a save failure still returns true (new aircraft).
func TestEnrich_SaveError(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockAircraftStore(ctrl)
	lookup := NewMockAircraftLookup(ctrl)

	info := &hexdb.AircraftInfo{ICAO24: "abc123", Type: "A320"}

	store.EXPECT().
		GetAircraftMeta(gomock.Any(), "abc123").
		Return(nil, nil)
	lookup.EXPECT().
		Lookup(gomock.Any(), "abc123").
		Return(info, nil)
	store.EXPECT().
		SaveAircraftMeta(gomock.Any(), info).
		Return(errors.New("write failed"))

	enr := New(lookup, store)
	got := enr.Enrich(context.Background(), "abc123")
	if got != true {
		t.Errorf("Enrich() = %v, want true even when save fails", got)
	}
}
