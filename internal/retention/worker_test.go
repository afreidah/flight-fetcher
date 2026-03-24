// -------------------------------------------------------------------------------
// Retention - Unit Tests
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Tests the retention cleanup worker: successful deletion, error handling,
// and no-op when nothing to delete.
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

// stubCleaner records calls and returns configured results.
type stubCleaner struct {
	sightingsDeleted int64
	alertsDeleted    int64
	routesDeleted    int64
	sightingsErr     error
	alertsErr        error
	routesErr        error
	calls            int
}

// DeleteOldSightings returns the stubbed result.
func (s *stubCleaner) DeleteOldSightings(_ context.Context, _ time.Duration) (int64, error) {
	s.calls++
	return s.sightingsDeleted, s.sightingsErr
}

// DeleteOldSquawkAlerts returns the stubbed result.
func (s *stubCleaner) DeleteOldSquawkAlerts(_ context.Context, _ time.Duration) (int64, error) {
	s.calls++
	return s.alertsDeleted, s.alertsErr
}

// DeleteOldRoutes returns the stubbed result.
func (s *stubCleaner) DeleteOldRoutes(_ context.Context, _ time.Duration) (int64, error) {
	s.calls++
	return s.routesDeleted, s.routesErr
}

// -------------------------------------------------------------------------
// TESTS
// -------------------------------------------------------------------------

// TestCleanup_DeletesOldRows verifies that cleanup deletes rows from both tables.
func TestCleanup_DeletesOldRows(t *testing.T) {
	c := &stubCleaner{sightingsDeleted: 100, alertsDeleted: 5}
	w := New(c, 30*24*time.Hour, 7*24*time.Hour, 24*time.Hour, time.Hour)
	w.cleanup(context.Background())

	if c.calls != 3 {
		t.Errorf("calls = %d, want 3", c.calls)
	}
}

// TestCleanup_NothingToDelete verifies that cleanup completes when no rows match.
func TestCleanup_NothingToDelete(t *testing.T) {
	c := &stubCleaner{sightingsDeleted: 0, alertsDeleted: 0}
	w := New(c, 30*24*time.Hour, 7*24*time.Hour, 24*time.Hour, time.Hour)
	w.cleanup(context.Background())

	if c.calls != 3 {
		t.Errorf("calls = %d, want 3", c.calls)
	}
}

// TestCleanup_SightingsError verifies that a sightings error does not stop alert cleanup.
func TestCleanup_SightingsError(t *testing.T) {
	c := &stubCleaner{sightingsErr: errors.New("pg down"), alertsDeleted: 3}
	w := New(c, 30*24*time.Hour, 7*24*time.Hour, 24*time.Hour, time.Hour)
	w.cleanup(context.Background())

	if c.calls != 3 {
		t.Errorf("calls = %d, want 3 (alerts should still be attempted)", c.calls)
	}
}

// TestCleanup_AlertsError verifies that an alerts error is handled gracefully.
func TestCleanup_AlertsError(t *testing.T) {
	c := &stubCleaner{sightingsDeleted: 50, alertsErr: errors.New("pg down")}
	w := New(c, 30*24*time.Hour, 7*24*time.Hour, 24*time.Hour, time.Hour)
	w.cleanup(context.Background())

	if c.calls != 3 {
		t.Errorf("calls = %d, want 3", c.calls)
	}
}
