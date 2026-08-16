// -------------------------------------------------------------------------------
// Retention - Package Documentation
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Package-level documentation and layering notes for the retention package.
// -------------------------------------------------------------------------------

// Package retention deletes aged-out sightings, squawk alerts, and cached
// routes from Postgres on a configurable interval, with a separate maximum age
// per table.
//
// The sighting log is the reason the worker exists: a busy receiver writes rows
// continuously, and without a ceiling the table grows without bound. Ages are
// per-table because the data ages differently - position history loses value
// quickly, while an emergency alert is worth keeping considerably longer.
//
// Each pass deletes in batches, looping until a batch comes back empty, rather
// than issuing one unbounded DELETE. A single statement covering a long backlog
// would hold locks long enough to interfere with the poller writing to the same
// tables. Failures are collected across all three tables and logged together,
// and the worker continues: cleanup falling behind is a capacity problem to
// alert on, not a reason to take the service down.
//
// retention sits in the orchestration layer. It imports runloop and declares a
// narrow view of the delete operations it needs rather than importing store.
package retention
