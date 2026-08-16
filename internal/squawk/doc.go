// -------------------------------------------------------------------------------
// Squawk - Package Documentation
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Package-level documentation and layering notes for the squawk package.
// -------------------------------------------------------------------------------

// Package squawk detects aircraft broadcasting emergency transponder codes -
// 7500 for hijack, 7600 for radio failure, 7700 for general emergency - and
// records, notifies, and enriches them.
//
// The monitor polls without geographic bounds rather than around the configured
// receiver location. An emergency anywhere is worth surfacing, and the codes
// are rare enough that a global scan returns almost nothing on a normal cycle.
//
// A transponder keeps broadcasting its code for as long as the condition
// lasts, so a naive scan would re-detect the same aircraft every cycle. A
// cooldown window guards against that: an alert is recorded only when no
// equivalent alert exists inside the window, which makes the alert table a log
// of distinct events rather than of scans. Notification and enrichment sit
// behind the same check, so an operator is paged once per emergency.
//
// Enrichment is dispatched to a background goroutine and its result ignored.
// Detection is the time-critical part; naming the aircraft is a convenience the
// dashboard can fill in late.
//
// The package also holds the Alert domain type, which the store persists and
// the dashboard server reads. Those two import squawk for the type alone and
// do not depend on the monitor.
//
// squawk sits in the orchestration layer. It imports geo, notify, and runloop
// and declares narrow views of its flight source, alert store, and enricher.
package squawk
