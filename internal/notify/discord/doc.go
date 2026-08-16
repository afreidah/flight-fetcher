// -------------------------------------------------------------------------------
// Discord - Package Documentation
//
// Project: Flight Fetcher / Author: Alex Freidah
//
// Package-level documentation and layering notes for the discord package.
// -------------------------------------------------------------------------------

// Package discord delivers notifications to a Discord channel through an
// incoming webhook, rendering each notify.Message as a rich embed with its
// typed fields laid out as embed fields.
//
// The webhook URL is the only credential and carries full posting rights for
// its channel, so it arrives from config rather than being constructed here.
//
// discord implements notify.Notifier and imports notify for the contract. It
// is registered with a notify.Manager by the composition root; nothing else in
// the service references it.
package discord
