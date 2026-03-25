// -------------------------------------------------------------------------------
// Retention - Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests the retention cleanup worker: successful deletion with batching,
// error handling, and no-op when nothing to delete.
// -------------------------------------------------------------------------------

package retention

import (
	"context"
	"errors"
	"testing"
	"time"
)

// -------------------------------------------------------------------------
// TEST HELPERS
// -------------------------------------------------------------------------

// stubCleaner records calls and returns configured results. Each delete
// method returns its configured value on the first call, then 0 on
// subsequent calls to simulate batched deletion completing.
type stubCleaner struct {
	sightingsDeleted int64
	alertsDeleted    int64
	routesDeleted    int64
	sightingsErr     error
	alertsErr        error
	routesErr        error

	sightingsCalls int
	alertsCalls    int
	routesCalls    int
}

// DeleteOldSightings returns the stubbed result on the first call, then 0.
func (s *stubCleaner) DeleteOldSightings(_ context.Context, _ time.Duration) (int64, error) {
	s.sightingsCalls++
	if s.sightingsErr != nil {
		return 0, s.sightingsErr
	}
	if s.sightingsCalls == 1 {
		return s.sightingsDeleted, nil
	}
	return 0, nil
}

// DeleteOldSquawkAlerts returns the stubbed result on the first call, then 0.
func (s *stubCleaner) DeleteOldSquawkAlerts(_ context.Context, _ time.Duration) (int64, error) {
	s.alertsCalls++
	if s.alertsErr != nil {
		return 0, s.alertsErr
	}
	if s.alertsCalls == 1 {
		return s.alertsDeleted, nil
	}
	return 0, nil
}

// DeleteOldRoutes returns the stubbed result on the first call, then 0.
func (s *stubCleaner) DeleteOldRoutes(_ context.Context, _ time.Duration) (int64, error) {
	s.routesCalls++
	if s.routesErr != nil {
		return 0, s.routesErr
	}
	if s.routesCalls == 1 {
		return s.routesDeleted, nil
	}
	return 0, nil
}

// -------------------------------------------------------------------------
// TESTS
// -------------------------------------------------------------------------

// TestCleanup_DeletesOldRows verifies that cleanup deletes rows from all tables via batched loop.
func TestCleanup_DeletesOldRows(t *testing.T) {
	c := &stubCleaner{sightingsDeleted: 100, alertsDeleted: 5}
	w := New(c, 30*24*time.Hour, 7*24*time.Hour, 24*time.Hour, time.Hour)
	w.cleanup(context.Background())

	// Each table gets called twice: once returning rows, once returning 0
	if c.sightingsCalls != 2 {
		t.Errorf("sightingsCalls = %d, want 2", c.sightingsCalls)
	}
	if c.alertsCalls != 2 {
		t.Errorf("alertsCalls = %d, want 2", c.alertsCalls)
	}
	if c.routesCalls != 1 {
		t.Errorf("routesCalls = %d, want 1 (0 rows on first call)", c.routesCalls)
	}
}

// TestCleanup_NothingToDelete verifies that cleanup completes when no rows match.
func TestCleanup_NothingToDelete(t *testing.T) {
	c := &stubCleaner{}
	w := New(c, 30*24*time.Hour, 7*24*time.Hour, 24*time.Hour, time.Hour)
	w.cleanup(context.Background())

	if c.sightingsCalls != 1 {
		t.Errorf("sightingsCalls = %d, want 1", c.sightingsCalls)
	}
}

// TestCleanup_SightingsError verifies that a sightings error does not stop alert cleanup.
func TestCleanup_SightingsError(t *testing.T) {
	c := &stubCleaner{sightingsErr: errors.New("pg down"), alertsDeleted: 3}
	w := New(c, 30*24*time.Hour, 7*24*time.Hour, 24*time.Hour, time.Hour)
	w.cleanup(context.Background())

	if c.sightingsCalls != 1 {
		t.Errorf("sightingsCalls = %d, want 1 (should stop on error)", c.sightingsCalls)
	}
	if c.alertsCalls != 2 {
		t.Errorf("alertsCalls = %d, want 2 (should still be attempted)", c.alertsCalls)
	}
}

// TestCleanup_AlertsError verifies that an alerts error is handled gracefully.
func TestCleanup_AlertsError(t *testing.T) {
	c := &stubCleaner{sightingsDeleted: 50, alertsErr: errors.New("pg down")}
	w := New(c, 30*24*time.Hour, 7*24*time.Hour, 24*time.Hour, time.Hour)
	w.cleanup(context.Background())

	if c.sightingsCalls != 2 {
		t.Errorf("sightingsCalls = %d, want 2", c.sightingsCalls)
	}
	if c.alertsCalls != 1 {
		t.Errorf("alertsCalls = %d, want 1 (should stop on error)", c.alertsCalls)
	}
}
