// -------------------------------------------------------------------------------
// RunLoop - Package Documentation
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Package-level documentation and layering notes for the runloop package.
// -------------------------------------------------------------------------------

// Package runloop provides the ticker loop shared by every background worker in
// the service: the poller, the squawk monitor, and the retention worker.
//
// Run invokes the given function once immediately and then on each tick until
// the context is cancelled. The immediate first call matters: a service with a
// long interval would otherwise sit idle through a full period after startup
// and appear dead. The function is run synchronously, so a call that outlasts
// the interval delays the next tick rather than overlapping with itself, and
// workers are expected to handle their own errors rather than returning them.
//
// runloop is a utility package. It imports nothing from this module and is
// imported by the orchestration layer.
package runloop
