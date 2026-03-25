// -------------------------------------------------------------------------------
// Enricher - Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests aircraft metadata enrichment and flight route enrichment: cache hits,
// new lookups, missing data, error handling, and disabled route enrichment.
// -------------------------------------------------------------------------------

package enricher

import (
	"context"
	"errors"
	"testing"

	"github.com/afreidah/flight-fetcher/internal/aircraft"
	"github.com/afreidah/flight-fetcher/internal/route"

	"go.uber.org/mock/gomock"
)

// -------------------------------------------------------------------------
// AIRCRAFT METADATA TESTS
// -------------------------------------------------------------------------

// TestEnrich_AlreadyCached verifies that a cached aircraft returns true (enrichment complete).
func TestEnrich_AlreadyCached(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockAircraftStore(ctrl)

	store.EXPECT().
		GetAircraftMeta(gomock.Any(), "abc123").
		Return(&aircraft.Info{ICAO24: "abc123"}, nil)

	enr := New(&Options{
		AircraftSources: []NamedSource[aircraft.Info]{{Name: "hexdb", Fn: failLookup[aircraft.Info](t)}},
		Store:           store,
	})
	got := enr.Enrich(context.Background(), "abc123")
	if got != true {
		t.Errorf("Enrich() = %v, want true for already cached aircraft", got)
	}
}

// TestEnrich_NewAircraft_LookupSuccess verifies that a new aircraft is looked up and saved.
func TestEnrich_NewAircraft_LookupSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockAircraftStore(ctrl)

	info := &aircraft.Info{
		ICAO24:           "abc123",
		Registration:     "N12345",
		ManufacturerName: "Boeing",
		Type:             "737-800",
		OperatorFlagCode: "UAL",
	}

	store.EXPECT().
		GetAircraftMeta(gomock.Any(), "abc123").
		Return(nil, nil)
	store.EXPECT().
		SaveAircraftMeta(gomock.Any(), info).
		Return(nil)

	enr := New(&Options{
		AircraftSources: []NamedSource[aircraft.Info]{{Name: "hexdb", Fn: staticLookup(info)}},
		Store:           store,
	})
	got := enr.Enrich(context.Background(), "abc123")
	if got != true {
		t.Errorf("Enrich() = %v, want true for new aircraft", got)
	}
}

// TestEnrich_NewAircraft_NotInHexDB verifies that a new aircraft not in HexDB still returns true.
func TestEnrich_NewAircraft_NotInHexDB(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockAircraftStore(ctrl)

	store.EXPECT().
		GetAircraftMeta(gomock.Any(), "abc123").
		Return(nil, nil)
	store.EXPECT().
		SaveAircraftMeta(gomock.Any(), &aircraft.Info{ICAO24: "abc123"}).
		Return(nil)

	enr := New(&Options{
		AircraftSources: []NamedSource[aircraft.Info]{{Name: "hexdb", Fn: staticLookup[aircraft.Info](nil)}},
		Store:           store,
	})
	got := enr.Enrich(context.Background(), "abc123")
	if got != true {
		t.Errorf("Enrich() = %v, want true for new aircraft not in hexdb", got)
	}
}

// TestEnrich_StoreGetError verifies that a store read failure returns false.
func TestEnrich_StoreGetError(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockAircraftStore(ctrl)

	store.EXPECT().
		GetAircraftMeta(gomock.Any(), "abc123").
		Return(nil, errors.New("db down"))

	enr := New(&Options{
		AircraftSources: []NamedSource[aircraft.Info]{{Name: "hexdb", Fn: failLookup[aircraft.Info](t)}},
		Store:           store,
	})
	got := enr.Enrich(context.Background(), "abc123")
	if got != false {
		t.Errorf("Enrich() = %v, want false when store read fails", got)
	}
}

// TestEnrich_LookupError verifies that when all sources fail, a sentinel is saved
// and true is returned (enrichment complete, no infinite retry).
func TestEnrich_LookupError(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockAircraftStore(ctrl)

	store.EXPECT().
		GetAircraftMeta(gomock.Any(), "abc123").
		Return(nil, nil)
	store.EXPECT().
		SaveAircraftMeta(gomock.Any(), &aircraft.Info{ICAO24: "abc123"}).
		Return(nil)

	enr := New(&Options{
		AircraftSources: []NamedSource[aircraft.Info]{{Name: "hexdb", Fn: errorLookup[aircraft.Info](errors.New("timeout"))}},
		Store:           store,
	})
	got := enr.Enrich(context.Background(), "abc123")
	if got != true {
		t.Errorf("Enrich() = %v, want true when all sources exhausted (sentinel saved)", got)
	}
}

// TestEnrich_SaveError verifies that a save failure still returns true (new aircraft).
func TestEnrich_SaveError(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockAircraftStore(ctrl)

	info := &aircraft.Info{ICAO24: "abc123", Type: "A320"}

	store.EXPECT().
		GetAircraftMeta(gomock.Any(), "abc123").
		Return(nil, nil)
	store.EXPECT().
		SaveAircraftMeta(gomock.Any(), info).
		Return(errors.New("write failed"))

	enr := New(&Options{
		AircraftSources: []NamedSource[aircraft.Info]{{Name: "hexdb", Fn: staticLookup(info)}},
		Store:           store,
	})
	got := enr.Enrich(context.Background(), "abc123")
	if got != true {
		t.Errorf("Enrich() = %v, want true even when save fails", got)
	}
}

// -------------------------------------------------------------------------
// ROUTE ENRICHMENT TESTS
// -------------------------------------------------------------------------

// TestEnrichRoute_Disabled verifies that EnrichRoute is a no-op when not configured.
func TestEnrichRoute_Disabled(t *testing.T) {
	enr := New(&Options{})
	enr.EnrichRoute(context.Background(), "AAL2079")
}

// TestEnrichRoute_AlreadyCached verifies that a cached route does not trigger a lookup.
func TestEnrichRoute_AlreadyCached(t *testing.T) {
	ctrl := gomock.NewController(t)
	routeStore := NewMockRouteStore(ctrl)

	routeStore.EXPECT().
		GetFlightRoute(gomock.Any(), "AAL2079").
		Return(&route.Info{FlightICAO: "AAL2079"}, nil)

	enr := New(&Options{
		RouteSources: []NamedSource[route.Info]{{Name: "airlabs", Fn: failLookup[route.Info](t)}},
		RouteStore:   routeStore,
	})
	enr.EnrichRoute(context.Background(), "AAL2079")
}

// TestEnrichRoute_NewRoute_LookupSuccess verifies that a new route is looked up and saved.
func TestEnrichRoute_NewRoute_LookupSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	routeStore := NewMockRouteStore(ctrl)

	rt := &route.Info{
		FlightICAO: "AAL2079",
		DepIATA:    "LAX",
		DepICAO:    "KLAX",
		ArrIATA:    "DFW",
		ArrICAO:    "KDFW",
	}

	routeStore.EXPECT().
		GetFlightRoute(gomock.Any(), "AAL2079").
		Return(nil, nil)
	routeStore.EXPECT().
		SaveFlightRoute(gomock.Any(), rt).
		Return(nil)

	enr := New(&Options{
		RouteSources: []NamedSource[route.Info]{{Name: "airlabs", Fn: staticLookup(rt)}},
		RouteStore:   routeStore,
	})
	enr.EnrichRoute(context.Background(), "AAL2079")
}

// TestEnrichRoute_NotFound verifies that a missing route does not cause an error.
func TestEnrichRoute_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	routeStore := NewMockRouteStore(ctrl)

	routeStore.EXPECT().
		GetFlightRoute(gomock.Any(), "AAL2079").
		Return(nil, nil)

	enr := New(&Options{
		RouteSources: []NamedSource[route.Info]{{Name: "airlabs", Fn: staticLookup[route.Info](nil)}},
		RouteStore:   routeStore,
	})
	enr.EnrichRoute(context.Background(), "AAL2079")
}

// TestEnrichRoute_LookupError verifies that a lookup failure is handled gracefully.
func TestEnrichRoute_LookupError(t *testing.T) {
	ctrl := gomock.NewController(t)
	routeStore := NewMockRouteStore(ctrl)

	routeStore.EXPECT().
		GetFlightRoute(gomock.Any(), "AAL2079").
		Return(nil, nil)

	enr := New(&Options{
		RouteSources: []NamedSource[route.Info]{{Name: "airlabs", Fn: errorLookup[route.Info](errors.New("timeout"))}},
		RouteStore:   routeStore,
	})
	enr.EnrichRoute(context.Background(), "AAL2079")
}

// TestEnrichRoute_SaveError verifies that a save failure is handled gracefully.
func TestEnrichRoute_SaveError(t *testing.T) {
	ctrl := gomock.NewController(t)
	routeStore := NewMockRouteStore(ctrl)

	rt := &route.Info{FlightICAO: "AAL2079", DepIATA: "LAX"}

	routeStore.EXPECT().
		GetFlightRoute(gomock.Any(), "AAL2079").
		Return(nil, nil)
	routeStore.EXPECT().
		SaveFlightRoute(gomock.Any(), rt).
		Return(errors.New("write failed"))

	enr := New(&Options{
		RouteSources: []NamedSource[route.Info]{{Name: "airlabs", Fn: staticLookup(rt)}},
		RouteStore:   routeStore,
	})
	enr.EnrichRoute(context.Background(), "AAL2079")
}

// TestEnrichRoute_StoreGetError verifies that a store read failure is handled gracefully.
func TestEnrichRoute_StoreGetError(t *testing.T) {
	ctrl := gomock.NewController(t)
	routeStore := NewMockRouteStore(ctrl)

	routeStore.EXPECT().
		GetFlightRoute(gomock.Any(), "AAL2079").
		Return(nil, errors.New("db down"))

	enr := New(&Options{
		RouteSources: []NamedSource[route.Info]{{Name: "airlabs", Fn: failLookup[route.Info](t)}},
		RouteStore:   routeStore,
	})
	enr.EnrichRoute(context.Background(), "AAL2079")
}

// -------------------------------------------------------------------------
// TEST HELPERS
// -------------------------------------------------------------------------

// staticLookup returns a lookup function that always returns the given value.
func staticLookup[T any](val *T) func(context.Context, string) (*T, error) {
	return func(context.Context, string) (*T, error) { return val, nil }
}

// errorLookup returns a lookup function that always returns the given error.
func errorLookup[T any](err error) func(context.Context, string) (*T, error) {
	return func(context.Context, string) (*T, error) { return nil, err }
}

// failLookup returns a lookup function that fails the test if called.
func failLookup[T any](t *testing.T) func(context.Context, string) (*T, error) {
	t.Helper()
	return func(context.Context, string) (*T, error) {
		t.Fatal("lookup should not have been called")
		return nil, nil
	}
}
